package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"expense-tracker/backend/dao"
	"expense-tracker/backend/request"
	"expense-tracker/backend/response"

	uuid "github.com/hashicorp/go-uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SyncService struct {
	DB *gorm.DB
}

type userGroupResolution struct {
	GroupID        string
	HadStoredGroup bool
}

func NewSyncService(db *gorm.DB) *SyncService {
	return &SyncService{DB: db}
}

func (s *SyncService) Sync(ctx context.Context, authUserID string, req *request.SyncRequest) (*response.SyncData, error) {
	clientGroupID := strings.TrimSpace(req.Settings.ActiveGroupID)
	authUserID = strings.TrimSpace(authUserID)
	if authUserID == "" {
		return nil, errors.New("authenticated user is required")
	}

	now := time.Now().UTC()
	result := &response.SyncData{}

	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		resolution, err := s.resolveUserGroup(tx, authUserID, clientGroupID, req.Settings.DeviceUserID, req.Groups, now)
		if err != nil {
			return fmt.Errorf("resolve user group: %w", err)
		}
		groupID := resolution.GroupID
		sourceGroupID := clientGroupID
		if sourceGroupID == "" {
			sourceGroupID = groupID
		}
		acceptIncoming := !(resolution.HadStoredGroup && sourceGroupID != groupID)

		if acceptIncoming {
			for _, account := range filterAccountsByGroup(req.Accounts, sourceGroupID, groupID) {
				if err := upsertAccount(tx, account); err != nil {
					return fmt.Errorf("upsert account %s: %w", account.ID, err)
				}
			}

			for _, category := range filterCategoriesByGroup(req.Categories, sourceGroupID, groupID, authUserID) {
				if err := upsertCategory(tx, category); err != nil {
					return fmt.Errorf("upsert category %s: %w", category.ID, err)
				}
			}

			for _, entry := range filterEntriesByGroup(req.Entries, sourceGroupID, groupID) {
				if err := upsertEntry(tx, entry); err != nil {
					return fmt.Errorf("upsert entry %s: %w", entry.ID, err)
				}

				if err := upsertMerchant(tx, merchantFromEntry(groupID, entry, now)); err != nil {
					return fmt.Errorf("upsert merchant from entry %s: %w", entry.ID, err)
				}
			}

			for _, adjustment := range filterAdjustmentsByGroup(req.Adjustments, sourceGroupID, groupID) {
				if err := upsertAdjustment(tx, adjustment); err != nil {
					return fmt.Errorf("upsert adjustment %s: %w", adjustment.ID, err)
				}
			}

			for _, merchant := range filterMerchantsByGroup(req.Merchants, sourceGroupID, groupID) {
				if err := upsertMerchant(tx, merchant); err != nil {
					return fmt.Errorf("upsert merchant %s: %w", merchant.ID, err)
				}
			}
		}

		groups, err := pullGroups(tx, groupID)
		if err != nil {
			return fmt.Errorf("pull groups: %w", err)
		}

		accounts, err := pullAccounts(tx, groupID)
		if err != nil {
			return fmt.Errorf("pull accounts: %w", err)
		}

		categories, err := pullCategories(tx, groupID, authUserID)
		if err != nil {
			return fmt.Errorf("pull categories: %w", err)
		}

		entries, err := pullEntries(tx, groupID)
		if err != nil {
			return fmt.Errorf("pull entries: %w", err)
		}

		adjustments, err := pullAdjustments(tx, groupID)
		if err != nil {
			return fmt.Errorf("pull adjustments: %w", err)
		}

		merchants, err := pullMerchants(tx, groupID)
		if err != nil {
			return fmt.Errorf("pull merchants: %w", err)
		}

		result.Groups = groups
		result.Accounts = accounts
		result.Categories = categories
		result.Entries = entries
		result.Adjustments = adjustments
		result.Merchants = merchants
		result.SyncedAt = now.Format(time.RFC3339Nano)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *SyncService) resolveUserGroup(
	tx *gorm.DB,
	authUserID string,
	clientGroupID string,
	deviceUserID string,
	payloadGroups []dao.ExpenseGroup,
	now time.Time,
) (*userGroupResolution, error) {
	user := &dao.ExpenseUser{}
	if err := tx.
		Where("id = ?::uuid and deleted_at is null", authUserID).
		First(user).Error; err != nil {
		return nil, err
	}

	if user.GroupID != nil && strings.TrimSpace(*user.GroupID) != "" {
		return &userGroupResolution{
			GroupID:        strings.TrimSpace(*user.GroupID),
			HadStoredGroup: true,
		}, nil
	}

	group := &dao.ExpenseGroup{}
	err := tx.Raw(fmt.Sprintf(`
		select id::text as id
		from %s
		where created_by = ?::uuid and deleted_at is null
		order by updated_at desc
		limit 1
	`, dao.QualifiedTable("expense_groups")), authUserID).Scan(group).Error
	if err != nil {
		return nil, err
	}
	targetGroupID := strings.TrimSpace(group.ID)
	if targetGroupID == "" {
		lastEntryGroup := struct {
			GroupID string `gorm:"column:group_id"`
		}{}
		err = tx.Raw(fmt.Sprintf(`
			select group_id::text as group_id
			from %s
			where created_by = ?::uuid and deleted_at is null
			order by updated_at desc
			limit 1
		`, dao.QualifiedTable("expense_entries")), authUserID).Scan(&lastEntryGroup).Error
		if err != nil {
			return nil, err
		}
		targetGroupID = strings.TrimSpace(lastEntryGroup.GroupID)
	}
	if targetGroupID == "" {
		targetGroupID = strings.TrimSpace(clientGroupID)
	}

	if targetGroupID == "" {
		generatedID, err := uuid.GenerateUUID()
		if err != nil {
			return nil, err
		}
		targetGroupID = generatedID
	}

	exists, err := groupExists(tx, targetGroupID)
	if err != nil {
		return nil, err
	}
	if !exists {
		group := activeGroup(payloadGroups, clientGroupID, deviceUserID, now)
		group.ID = targetGroupID
		group.InviteCode = randomInviteCode(targetGroupID)
		if err := upsertGroup(tx, group); err != nil {
			return nil, fmt.Errorf("create group: %w", err)
		}
	}

	if err := tx.Model(&dao.ExpenseUser{}).
		Where("id = ?::uuid", authUserID).
		Updates(map[string]any{
			"group_id":   targetGroupID,
			"updated_at": now,
		}).Error; err != nil {
		return nil, err
	}

	return &userGroupResolution{
		GroupID:        targetGroupID,
		HadStoredGroup: false,
	}, nil
}

func activeGroup(groups []dao.ExpenseGroup, groupID, deviceUserID string, now time.Time) dao.ExpenseGroup {
	for _, group := range groups {
		if group.ID == groupID {
			group.CreatedAt = normalizeTime(group.CreatedAt, now)
			group.UpdatedAt = normalizeTime(group.UpdatedAt, now)
			if group.CreatedBy == nil && deviceUserID != "" {
				group.CreatedBy = &deviceUserID
			}
			return group
		}
	}

	return dao.ExpenseGroup{
		ID:         groupID,
		Name:       "Household",
		InviteCode: randomInviteCode(groupID),
		CreatedBy:  optional(deviceUserID),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func upsertGroup(tx *gorm.DB, group dao.ExpenseGroup) error {
	row := map[string]any{
		"id":          group.ID,
		"name":        group.Name,
		"invite_code": group.InviteCode,
		"created_by":  stringOrNil(group.CreatedBy),
		"created_at":  normalizeTime(group.CreatedAt, time.Now().UTC()),
		"updated_at":  normalizeTime(group.UpdatedAt, time.Now().UTC()),
		"deleted_at":  group.DeletedAt,
	}

	return tx.Table((dao.ExpenseGroup{}).TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"name":        row["name"],
				"invite_code": row["invite_code"],
				"created_by":  gorm.Expr(fmt.Sprintf("coalesce(%s.created_by, excluded.created_by)", dao.QualifiedTable("expense_groups"))),
				"updated_at":  row["updated_at"],
				"deleted_at":  row["deleted_at"],
			}),
		}).
		Create(row).Error
}

func upsertAccount(tx *gorm.DB, account dao.ExpenseAccount) error {
	now := time.Now().UTC()
	row := map[string]any{
		"id":              account.ID,
		"group_id":        account.GroupID,
		"name":            account.Name,
		"type":            account.Type,
		"opening_balance": account.OpeningBalance,
		"color":           account.Color,
		"icon":            normalizeIcon(account.Icon, defaultAccountIcon(account.Type)),
		"created_at":      normalizeTime(account.CreatedAt, now),
		"updated_at":      normalizeTime(account.UpdatedAt, now),
		"deleted_at":      account.DeletedAt,
	}

	return tx.Table((dao.ExpenseAccount{}).TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"group_id":        row["group_id"],
				"name":            row["name"],
				"type":            row["type"],
				"opening_balance": row["opening_balance"],
				"color":           row["color"],
				"icon":            row["icon"],
				"updated_at":      row["updated_at"],
				"deleted_at":      row["deleted_at"],
			}),
		}).
		Create(row).Error
}

func upsertCategory(tx *gorm.DB, category dao.ExpenseCategory) error {
	now := time.Now().UTC()
	scope := normalizeCategoryScope(category.Scope)
	var ownerUserID any
	if scope == "user" {
		ownerUserID = stringOrNil(category.OwnerUserID)
	}
	row := map[string]any{
		"id":             category.ID,
		"group_id":       category.GroupID,
		"name":           category.Name,
		"type":           normalizeCategoryType(category.Type, category.Name),
		"scope":          scope,
		"owner_user_id":  ownerUserID,
		"color":          category.Color,
		"icon":           normalizeIcon(category.Icon, defaultCategoryIcon(category.Name)),
		"monthly_target": category.MonthlyTarget,
		"created_at":     normalizeTime(category.CreatedAt, now),
		"updated_at":     normalizeTime(category.UpdatedAt, now),
		"deleted_at":     category.DeletedAt,
	}

	return tx.Table((dao.ExpenseCategory{}).TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"group_id":       row["group_id"],
				"name":           row["name"],
				"type":           row["type"],
				"scope":          row["scope"],
				"owner_user_id":  row["owner_user_id"],
				"color":          row["color"],
				"icon":           row["icon"],
				"monthly_target": row["monthly_target"],
				"updated_at":     row["updated_at"],
				"deleted_at":     row["deleted_at"],
			}),
		}).
		Create(row).Error
}

func upsertEntry(tx *gorm.DB, entry dao.ExpenseEntry) error {
	now := time.Now().UTC()
	metadataJSON, err := json.Marshal(entry.Metadata)
	if err != nil {
		return fmt.Errorf("marshal entry metadata: %w", err)
	}
	if len(metadataJSON) == 0 || string(metadataJSON) == "null" {
		metadataJSON = []byte("{}")
	}
	table := dao.QualifiedTable("expense_entries")

	return tx.Exec(fmt.Sprintf(`
		insert into %s (
			id, group_id, account_id, category_id, type, amount, currency, occurred_on,
			merchant, note, metadata, created_by, created_at, updated_at, deleted_at
		) values (
			?::uuid, ?::uuid, ?::uuid, ?::uuid, ?, ?, ?, ?::date,
			?, ?, ?::jsonb, ?::uuid, ?, ?, ?
		)
		on conflict (id) do update set
			group_id = excluded.group_id,
			account_id = excluded.account_id,
			category_id = excluded.category_id,
			type = excluded.type,
			amount = excluded.amount,
			currency = excluded.currency,
			occurred_on = excluded.occurred_on,
			merchant = excluded.merchant,
			note = excluded.note,
			metadata = excluded.metadata,
			created_by = coalesce(%s.created_by, excluded.created_by),
			updated_at = excluded.updated_at,
			deleted_at = excluded.deleted_at
	`, table, table),
		entry.ID,
		entry.GroupID,
		entry.AccountID,
		entry.CategoryID,
		entry.Type,
		entry.Amount,
		normalizeEntryCurrency(entry.Currency),
		entry.OccurredOn,
		entry.Merchant,
		entry.Note,
		string(metadataJSON),
		stringOrNil(entry.CreatedBy),
		normalizeTime(entry.CreatedAt, now),
		normalizeTime(entry.UpdatedAt, now),
		entry.DeletedAt,
	).Error
}

func upsertAdjustment(tx *gorm.DB, adjustment dao.ExpenseCategoryAdjustment) error {
	now := time.Now().UTC()
	row := map[string]any{
		"id":          adjustment.ID,
		"group_id":    adjustment.GroupID,
		"category_id": adjustment.CategoryID,
		"amount":      adjustment.Amount,
		"occurred_on": adjustment.OccurredOn,
		"note":        adjustment.Note,
		"created_at":  normalizeTime(adjustment.CreatedAt, now),
		"updated_at":  normalizeTime(adjustment.UpdatedAt, now),
		"deleted_at":  adjustment.DeletedAt,
	}

	return tx.Table((dao.ExpenseCategoryAdjustment{}).TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"group_id":    row["group_id"],
				"category_id": row["category_id"],
				"amount":      row["amount"],
				"occurred_on": row["occurred_on"],
				"note":        row["note"],
				"updated_at":  row["updated_at"],
				"deleted_at":  row["deleted_at"],
			}),
		}).
		Create(row).Error
}

func upsertMerchant(tx *gorm.DB, merchant dao.ExpenseMerchant) error {
	now := time.Now().UTC()
	name := strings.TrimSpace(merchant.Name)
	if name == "" {
		return nil
	}

	normalizedName := normalizeMerchantName(name)
	if normalizedName == "" {
		return nil
	}

	merchantID := merchant.ID
	if merchantID == "" {
		generatedID, err := uuid.GenerateUUID()
		if err != nil {
			return err
		}
		merchantID = generatedID
	}

	usageCount := merchant.UsageCount
	if usageCount <= 0 {
		usageCount = 1
	}

	row := map[string]any{
		"id":              merchantID,
		"group_id":        merchant.GroupID,
		"name":            name,
		"normalized_name": normalizedName,
		"usage_count":     usageCount,
		"last_used_at":    merchant.LastUsedAt,
		"created_at":      normalizeTime(merchant.CreatedAt, now),
		"updated_at":      normalizeTime(merchant.UpdatedAt, now),
		"deleted_at":      merchant.DeletedAt,
	}

	return tx.Table((dao.ExpenseMerchant{}).TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "group_id"}, {Name: "normalized_name"}},
			DoUpdates: clause.Assignments(map[string]any{
				"name":         row["name"],
				"usage_count":  gorm.Expr(fmt.Sprintf("greatest(%s.usage_count, excluded.usage_count)", dao.QualifiedTable("expense_merchants"))),
				"last_used_at": gorm.Expr(fmt.Sprintf("coalesce(greatest(%s.last_used_at, excluded.last_used_at), %s.last_used_at, excluded.last_used_at)", dao.QualifiedTable("expense_merchants"), dao.QualifiedTable("expense_merchants"))),
				"updated_at":   row["updated_at"],
				"deleted_at":   row["deleted_at"],
			}),
		}).
		Create(row).Error
}

func pullGroups(tx *gorm.DB, groupID string) ([]dao.ExpenseGroup, error) {
	var rows []dao.ExpenseGroup
	if err := tx.Raw(fmt.Sprintf(`
		select
			id::text as id,
			name,
			invite_code,
			created_by::text as created_by,
			created_at,
			updated_at,
			deleted_at
		from %s
		where id = ?::uuid
	`, dao.QualifiedTable("expense_groups")), groupID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func pullAccounts(tx *gorm.DB, groupID string) ([]dao.ExpenseAccount, error) {
	var rows []dao.ExpenseAccount
	if err := tx.Raw(fmt.Sprintf(`
		select
			id::text as id,
			group_id::text as group_id,
			name,
			type,
			opening_balance,
			color,
			icon,
			created_at,
			updated_at,
			deleted_at
		from %s
		where group_id = ?::uuid
		order by updated_at desc
	`, dao.QualifiedTable("expense_accounts")), groupID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func pullCategories(tx *gorm.DB, groupID string, authUserID string) ([]dao.ExpenseCategory, error) {
	var rows []dao.ExpenseCategory
	authUserID = strings.TrimSpace(authUserID)
	accessClause := "(scope = 'household')"
	args := []any{groupID}
	if authUserID != "" {
		accessClause = "(scope = 'household' or (scope = 'user' and owner_user_id = ?::uuid))"
		args = append(args, authUserID)
	}
	if err := tx.Raw(fmt.Sprintf(`
		select
			id::text as id,
			group_id::text as group_id,
			name,
			type,
			scope,
			owner_user_id::text as owner_user_id,
			color,
			icon,
			monthly_target,
			created_at,
			updated_at,
			deleted_at
		from %s
		where group_id = ?::uuid and %s
		order by updated_at desc
	`, dao.QualifiedTable("expense_categories"), accessClause), args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func pullEntries(tx *gorm.DB, groupID string) ([]dao.ExpenseEntry, error) {
	var rows []dao.ExpenseEntry
	if err := tx.Raw(fmt.Sprintf(`
		select
			id::text as id,
			group_id::text as group_id,
			account_id::text as account_id,
			category_id::text as category_id,
			type,
			amount,
			currency,
			to_char(occurred_on, 'YYYY-MM-DD') as occurred_on,
			merchant,
			note,
			metadata,
			created_by::text as created_by,
			created_at,
			updated_at,
			deleted_at
		from %s
		where group_id = ?::uuid
		order by occurred_on desc, updated_at desc
	`, dao.QualifiedTable("expense_entries")), groupID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func pullAdjustments(tx *gorm.DB, groupID string) ([]dao.ExpenseCategoryAdjustment, error) {
	var rows []dao.ExpenseCategoryAdjustment
	if err := tx.Raw(fmt.Sprintf(`
		select
			id::text as id,
			group_id::text as group_id,
			category_id::text as category_id,
			amount,
			to_char(occurred_on, 'YYYY-MM-DD') as occurred_on,
			note,
			created_at,
			updated_at,
			deleted_at
		from %s
		where group_id = ?::uuid
		order by occurred_on desc, updated_at desc
	`, dao.QualifiedTable("expense_category_adjustments")), groupID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func pullMerchants(tx *gorm.DB, groupID string) ([]dao.ExpenseMerchant, error) {
	var rows []dao.ExpenseMerchant
	if err := tx.Raw(fmt.Sprintf(`
		select
			id::text as id,
			group_id::text as group_id,
			name,
			normalized_name,
			usage_count,
			last_used_at,
			created_at,
			updated_at,
			deleted_at
		from %s
		where group_id = ?::uuid
		order by usage_count desc, updated_at desc
	`, dao.QualifiedTable("expense_merchants")), groupID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func groupExists(tx *gorm.DB, groupID string) (bool, error) {
	var count int64
	err := tx.Table((dao.ExpenseGroup{}).TableName()).
		Where("id = ?::uuid and deleted_at is null", groupID).
		Count(&count).Error
	return count > 0, err
}

func filterAccountsByGroup(records []dao.ExpenseAccount, sourceGroupID string, targetGroupID string) []dao.ExpenseAccount {
	filtered := make([]dao.ExpenseAccount, 0, len(records))
	for _, record := range records {
		if record.GroupID == sourceGroupID {
			record.GroupID = targetGroupID
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func filterCategoriesByGroup(records []dao.ExpenseCategory, sourceGroupID string, targetGroupID string, authUserID string) []dao.ExpenseCategory {
	filtered := make([]dao.ExpenseCategory, 0, len(records))
	authUserID = strings.TrimSpace(authUserID)
	for _, record := range records {
		if record.GroupID != sourceGroupID {
			continue
		}
		record.GroupID = targetGroupID

		record.Scope = normalizeCategoryScope(record.Scope)
		if record.Scope == "user" {
			if authUserID == "" {
				continue
			}
			record.OwnerUserID = optional(authUserID)
		} else {
			record.OwnerUserID = nil
		}

		filtered = append(filtered, record)
	}
	return filtered
}

func filterEntriesByGroup(records []dao.ExpenseEntry, sourceGroupID string, targetGroupID string) []dao.ExpenseEntry {
	filtered := make([]dao.ExpenseEntry, 0, len(records))
	for _, record := range records {
		if record.GroupID == sourceGroupID {
			record.GroupID = targetGroupID
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func filterAdjustmentsByGroup(records []dao.ExpenseCategoryAdjustment, sourceGroupID string, targetGroupID string) []dao.ExpenseCategoryAdjustment {
	filtered := make([]dao.ExpenseCategoryAdjustment, 0, len(records))
	for _, record := range records {
		if record.GroupID == sourceGroupID {
			record.GroupID = targetGroupID
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func filterMerchantsByGroup(records []dao.ExpenseMerchant, sourceGroupID string, targetGroupID string) []dao.ExpenseMerchant {
	filtered := make([]dao.ExpenseMerchant, 0, len(records))
	for _, record := range records {
		if record.GroupID == sourceGroupID {
			record.GroupID = targetGroupID
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func normalizeTime(value, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringOrNil(value *string) any {
	if value == nil || *value == "" {
		return nil
	}
	return *value
}

func randomInviteCode(groupID string) string {
	if len(groupID) >= 6 {
		return groupID[:6]
	}
	return "GROUP1"
}

func merchantFromEntry(groupID string, entry dao.ExpenseEntry, now time.Time) dao.ExpenseMerchant {
	usedAt := now
	if parsed, err := time.Parse("2006-01-02", entry.OccurredOn); err == nil {
		usedAt = parsed.UTC()
	}

	return dao.ExpenseMerchant{
		GroupID:        groupID,
		Name:           entry.Merchant,
		NormalizedName: normalizeMerchantName(entry.Merchant),
		UsageCount:     1,
		LastUsedAt:     &usedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func normalizeMerchantName(value string) string {
	compact := strings.Join(strings.Fields(strings.ToLower(value)), " ")
	return compact
}

func normalizeEntryCurrency(value string) string {
	_ = strings.TrimSpace(value)
	return "SGD"
}
