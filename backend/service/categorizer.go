package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"expense-tracker/backend/dao"

	"github.com/apex/log"
	uuid "github.com/hashicorp/go-uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const fuzzyMerchantThreshold = 0.42

type merchantCategorySuggestion struct {
	CategoryID string
	Source     string
	Confidence float64
}

func (s *ExpenseService) suggestCategoryForEntry(
	ctx context.Context,
	groupID string,
	authUserID string,
	entryType string,
	accountID string,
	merchant string,
	note string,
) (*merchantCategorySuggestion, error) {
	normalizedType := normalizeCategoryType(entryType, "")
	merchantKey := normalizeMerchantKey(merchant)
	noteKey := strings.ToLower(strings.TrimSpace(note))

	if merchantKey != "" {
		if fromMap, err := s.lookupMerchantCategoryMap(ctx, groupID, merchantKey, normalizedType); err != nil {
			return nil, err
		} else if fromMap != nil {
			log.WithFields(log.Fields{
				"groupId":      groupID,
				"userId":       authUserID,
				"merchant":     merchant,
				"merchantKey":  merchantKey,
				"entryType":    normalizedType,
				"categoryId":   fromMap.CategoryID,
				"confidence":   fromMap.Confidence,
				"source":       fromMap.Source,
				"stage":        "merchant-map",
				"accountId":    accountID,
				"noteProvided": strings.TrimSpace(note) != "",
			}).Info("categorizer selected category")
			return fromMap, nil
		}
	}

	if fromRules, err := s.lookupCategoryRules(ctx, groupID, normalizedType, accountID, merchantKey, noteKey); err != nil {
		return nil, err
	} else if fromRules != nil {
		log.WithFields(log.Fields{
			"groupId":      groupID,
			"userId":       authUserID,
			"merchant":     merchant,
			"merchantKey":  merchantKey,
			"entryType":    normalizedType,
			"categoryId":   fromRules.CategoryID,
			"confidence":   fromRules.Confidence,
			"source":       fromRules.Source,
			"stage":        "rules",
			"accountId":    accountID,
			"noteProvided": strings.TrimSpace(note) != "",
		}).Info("categorizer selected category")
		return fromRules, nil
	}

	if merchantKey != "" {
		if fromFuzzy, err := s.lookupFuzzyMerchantCategory(ctx, groupID, merchantKey, normalizedType); err != nil {
			return nil, err
		} else if fromFuzzy != nil {
			log.WithFields(log.Fields{
				"groupId":      groupID,
				"userId":       authUserID,
				"merchant":     merchant,
				"merchantKey":  merchantKey,
				"entryType":    normalizedType,
				"categoryId":   fromFuzzy.CategoryID,
				"confidence":   fromFuzzy.Confidence,
				"source":       fromFuzzy.Source,
				"stage":        "fuzzy",
				"accountId":    accountID,
				"noteProvided": strings.TrimSpace(note) != "",
			}).Info("categorizer selected category")
			return fromFuzzy, nil
		}
	}

	log.WithFields(log.Fields{
		"groupId":      groupID,
		"userId":       authUserID,
		"merchant":     merchant,
		"merchantKey":  merchantKey,
		"entryType":    normalizedType,
		"accountId":    accountID,
		"noteProvided": strings.TrimSpace(note) != "",
	}).Debug("categorizer found no category match")

	return nil, nil
}

func (s *ExpenseService) lookupMerchantCategoryMap(ctx context.Context, groupID string, merchantKey string, entryType string) (*merchantCategorySuggestion, error) {
	var result merchantCategorySuggestion
	err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select
			category_id::text as category_id,
			coalesce(source, 'exact') as source,
			coalesce(confidence, 1.0) as confidence
		from %s
		where group_id = ?::uuid
			and normalized_merchant = ?
			and entry_type = ?
			and deleted_at is null
		order by hit_count desc, updated_at desc
		limit 1
	`, dao.QualifiedTable("expense_merchant_category_maps")), groupID, merchantKey, entryType).Scan(&result).Error
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.CategoryID) == "" {
		return nil, nil
	}
	if result.Source == "" {
		result.Source = "exact"
	}
	return &result, nil
}

func (s *ExpenseService) lookupFuzzyMerchantCategory(ctx context.Context, groupID string, merchantKey string, entryType string) (*merchantCategorySuggestion, error) {
	type fuzzyMatch struct {
		CategoryID string
		Score      float64
	}

	var best fuzzyMatch
	err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select
			category_id::text as category_id,
			similarity(normalized_merchant, ?) as score
		from %s
		where group_id = ?::uuid
			and entry_type = ?
			and deleted_at is null
			and normalized_merchant %% ?
		order by score desc, hit_count desc
		limit 1
	`, dao.QualifiedTable("expense_merchant_category_maps")), merchantKey, groupID, entryType, merchantKey).Scan(&best).Error
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(best.CategoryID) == "" || best.Score < fuzzyMerchantThreshold {
		return nil, nil
	}

	confidence := 0.55 + (best.Score * 0.35)
	if confidence > 0.9 {
		confidence = 0.9
	}

	return &merchantCategorySuggestion{
		CategoryID: best.CategoryID,
		Source:     "fuzzy",
		Confidence: confidence,
	}, nil
}

func (s *ExpenseService) lookupCategoryRules(
	ctx context.Context,
	groupID string,
	entryType string,
	accountID string,
	merchantKey string,
	noteKey string,
) (*merchantCategorySuggestion, error) {
	rules := make([]dao.ExpenseCategoryRule, 0)
	err := s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
		select
			id::text as id,
			group_id::text as group_id,
			priority,
			enabled,
			entry_type,
			match_field,
			match_kind,
			pattern,
			category_id::text as category_id,
			coalesce(confidence, 0.9) as confidence,
			created_at,
			updated_at,
			deleted_at
		from %s
		where group_id = ?::uuid
			and enabled = true
			and deleted_at is null
			and entry_type in ('any', ?)
		order by priority asc, updated_at desc
	`, dao.QualifiedTable("expense_category_rules")), groupID, entryType).Scan(&rules).Error
	if err != nil {
		return nil, err
	}

	accountType := ""
	if strings.TrimSpace(accountID) != "" {
		_ = s.DB.WithContext(ctx).Raw(fmt.Sprintf(`
			select type
			from %s
			where id = ?::uuid and group_id = ?::uuid and deleted_at is null
			limit 1
		`, dao.QualifiedTable("expense_accounts")), accountID, groupID).Scan(&accountType).Error
		accountType = strings.ToLower(strings.TrimSpace(accountType))
	}

	for _, rule := range rules {
		pattern := strings.TrimSpace(rule.Pattern)
		if pattern == "" {
			continue
		}

		target := ""
		switch strings.ToLower(strings.TrimSpace(rule.MatchField)) {
		case "note":
			target = noteKey
		case "account_type":
			target = accountType
		default:
			target = merchantKey
		}
		if target == "" {
			continue
		}

		if matchesRule(strings.ToLower(strings.TrimSpace(rule.MatchKind)), pattern, target) {
			confidence := rule.Confidence
			if confidence <= 0 {
				confidence = 0.9
			}
			return &merchantCategorySuggestion{
				CategoryID: rule.CategoryID,
				Source:     "rule",
				Confidence: confidence,
			}, nil
		}
	}

	return nil, nil
}

func matchesRule(matchKind string, pattern string, target string) bool {
	pattern = strings.TrimSpace(pattern)
	target = strings.TrimSpace(target)
	if pattern == "" || target == "" {
		return false
	}

	patternLower := strings.ToLower(pattern)
	targetLower := strings.ToLower(target)

	switch matchKind {
	case "equals":
		return targetLower == patternLower
	case "prefix":
		return strings.HasPrefix(targetLower, patternLower)
	case "regex":
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(target)
	default:
		return strings.Contains(targetLower, patternLower)
	}
}

func (s *ExpenseService) learnMerchantCategory(
	tx *gorm.DB,
	groupID string,
	merchant string,
	entryType string,
	categoryID string,
	confidence float64,
	source string,
	when time.Time,
) error {
	merchantKey := normalizeMerchantKey(merchant)
	if merchantKey == "" || strings.TrimSpace(categoryID) == "" {
		return nil
	}

	merchantCategoryMapID, err := uuid.GenerateUUID()
	if err != nil {
		return err
	}
	if confidence <= 0 {
		confidence = 1.0
	}
	if source == "" {
		source = "learned"
	}

	row := map[string]any{
		"id":                  merchantCategoryMapID,
		"group_id":            groupID,
		"normalized_merchant": merchantKey,
		"entry_type":          normalizeCategoryType(entryType, ""),
		"category_id":         strings.TrimSpace(categoryID),
		"confidence":          confidence,
		"source":              source,
		"hit_count":           1,
		"last_seen_at":        when,
		"created_at":          when,
		"updated_at":          when,
		"deleted_at":          nil,
	}

	if err := tx.Table((dao.ExpenseMerchantCategoryMap{}).TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "group_id"},
				{Name: "normalized_merchant"},
				{Name: "entry_type"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"category_id":  row["category_id"],
				"confidence":   row["confidence"],
				"source":       row["source"],
				"hit_count":    gorm.Expr(fmt.Sprintf("%s.hit_count + 1", dao.QualifiedTable("expense_merchant_category_maps"))),
				"last_seen_at": row["last_seen_at"],
				"updated_at":   row["updated_at"],
				"deleted_at":   nil,
			}),
		}).Create(row).Error; err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"groupId":     groupID,
		"merchant":    merchant,
		"merchantKey": merchantKey,
		"entryType":   row["entry_type"],
		"categoryId":  row["category_id"],
		"confidence":  row["confidence"],
		"source":      row["source"],
		"lastSeenAt":  row["last_seen_at"],
		"operation":   "upsert",
		"targetTable": (dao.ExpenseMerchantCategoryMap{}).TableName(),
	}).Info("categorizer learned merchant category mapping")

	return nil
}

func normalizeMerchantKey(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}

	normalized = strings.Join(strings.Fields(normalized), " ")
	re := regexp.MustCompile(`[^\p{L}\p{N}\s]`)
	normalized = re.ReplaceAllString(normalized, " ")
	normalized = strings.Join(strings.Fields(normalized), " ")

	stopWords := []string{"pte", "ltd", "singapore", "sg", "store", "shop"}
	tokens := make([]string, 0, len(strings.Fields(normalized)))
	for _, token := range strings.Fields(normalized) {
		keep := true
		for _, stop := range stopWords {
			if token == stop {
				keep = false
				break
			}
		}
		if keep {
			tokens = append(tokens, token)
		}
	}

	return strings.Join(tokens, " ")
}
