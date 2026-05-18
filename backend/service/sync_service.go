package service

import (
	"context"
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

func NewSyncService(db *gorm.DB) *SyncService {
	return &SyncService{DB: db}
}

func (s *SyncService) Sync(ctx context.Context, req *request.SyncRequest) (*response.SyncData, error) {
	groupID := req.Settings.ActiveGroupID
	if groupID == "" {
		return nil, errors.New("settings.activeGroupId is required")
	}

	now := time.Now().UTC()
	group := activeGroup(req.Groups, groupID, req.Settings.DeviceUserID, now)
	result := &response.SyncData{}

	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := upsertGroup(tx, group); err != nil {
			return fmt.Errorf("upsert group: %w", err)
		}

		for _, account := range filterAccountsByGroup(req.Accounts, groupID) {
			if err := upsertAccount(tx, account); err != nil {
				return fmt.Errorf("upsert account %s: %w", account.ID, err)
			}
		}

		for _, category := range filterCategoriesByGroup(req.Categories, groupID) {
			if err := upsertCategory(tx, category); err != nil {
				return fmt.Errorf("upsert category %s: %w", category.ID, err)
			}
		}

		for _, entry := range filterEntriesByGroup(req.Entries, groupID) {
			if err := upsertEntry(tx, entry); err != nil {
				return fmt.Errorf("upsert entry %s: %w", entry.ID, err)
			}

			if err := upsertMerchant(tx, merchantFromEntry(groupID, entry, now)); err != nil {
				return fmt.Errorf("upsert merchant from entry %s: %w", entry.ID, err)
			}
		}

		for _, adjustment := range filterAdjustmentsByGroup(req.Adjustments, groupID) {
			if err := upsertAdjustment(tx, adjustment); err != nil {
				return fmt.Errorf("upsert adjustment %s: %w", adjustment.ID, err)
			}
		}

		for _, merchant := range filterMerchantsByGroup(req.Merchants, groupID) {
			if err := upsertMerchant(tx, merchant); err != nil {
				return fmt.Errorf("upsert merchant %s: %w", merchant.ID, err)
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

		categories, err := pullCategories(tx, groupID)
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
				"created_by":  gorm.Expr("coalesce(public.expense_groups.created_by, excluded.created_by)"),
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
	row := map[string]any{
		"id":             category.ID,
		"group_id":       category.GroupID,
		"name":           category.Name,
		"type":           normalizeCategoryType(category.Type, category.Name),
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
	row := map[string]any{
		"id":          entry.ID,
		"group_id":    entry.GroupID,
		"account_id":  entry.AccountID,
		"category_id": entry.CategoryID,
		"type":        entry.Type,
		"amount":      entry.Amount,
		"currency":    normalizeEntryCurrency(entry.Currency),
		"occurred_on": entry.OccurredOn,
		"merchant":    entry.Merchant,
		"note":        entry.Note,
		"created_by":  stringOrNil(entry.CreatedBy),
		"created_at":  normalizeTime(entry.CreatedAt, now),
		"updated_at":  normalizeTime(entry.UpdatedAt, now),
		"deleted_at":  entry.DeletedAt,
	}

	return tx.Table((dao.ExpenseEntry{}).TableName()).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"group_id":    row["group_id"],
				"account_id":  row["account_id"],
				"category_id": row["category_id"],
				"type":        row["type"],
				"amount":      row["amount"],
				"currency":    row["currency"],
				"occurred_on": row["occurred_on"],
				"merchant":    row["merchant"],
				"note":        row["note"],
				"created_by":  gorm.Expr("coalesce(public.expense_entries.created_by, excluded.created_by)"),
				"updated_at":  row["updated_at"],
				"deleted_at":  row["deleted_at"],
			}),
		}).
		Create(row).Error
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
				"usage_count":  gorm.Expr("greatest(public.expense_merchants.usage_count, excluded.usage_count)"),
				"last_used_at": gorm.Expr("coalesce(greatest(public.expense_merchants.last_used_at, excluded.last_used_at), public.expense_merchants.last_used_at, excluded.last_used_at)"),
				"updated_at":   row["updated_at"],
				"deleted_at":   row["deleted_at"],
			}),
		}).
		Create(row).Error
}

func pullGroups(tx *gorm.DB, groupID string) ([]dao.ExpenseGroup, error) {
	var rows []dao.ExpenseGroup
	if err := tx.Raw(`
		select
			id::text as id,
			name,
			invite_code,
			created_by::text as created_by,
			created_at,
			updated_at,
			deleted_at
		from public.expense_groups
		where id = ?::uuid
	`, groupID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func pullAccounts(tx *gorm.DB, groupID string) ([]dao.ExpenseAccount, error) {
	var rows []dao.ExpenseAccount
	if err := tx.Raw(`
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
		from public.expense_accounts
		where group_id = ?::uuid
		order by updated_at desc
	`, groupID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func pullCategories(tx *gorm.DB, groupID string) ([]dao.ExpenseCategory, error) {
	var rows []dao.ExpenseCategory
	if err := tx.Raw(`
		select
			id::text as id,
			group_id::text as group_id,
			name,
			type,
			color,
			icon,
			monthly_target,
			created_at,
			updated_at,
			deleted_at
		from public.expense_categories
		where group_id = ?::uuid
		order by updated_at desc
	`, groupID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func pullEntries(tx *gorm.DB, groupID string) ([]dao.ExpenseEntry, error) {
	var rows []dao.ExpenseEntry
	if err := tx.Raw(`
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
			created_by::text as created_by,
			created_at,
			updated_at,
			deleted_at
		from public.expense_entries
		where group_id = ?::uuid
		order by occurred_on desc, updated_at desc
	`, groupID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func pullAdjustments(tx *gorm.DB, groupID string) ([]dao.ExpenseCategoryAdjustment, error) {
	var rows []dao.ExpenseCategoryAdjustment
	if err := tx.Raw(`
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
		from public.expense_category_adjustments
		where group_id = ?::uuid
		order by occurred_on desc, updated_at desc
	`, groupID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func pullMerchants(tx *gorm.DB, groupID string) ([]dao.ExpenseMerchant, error) {
	var rows []dao.ExpenseMerchant
	if err := tx.Raw(`
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
		from public.expense_merchants
		where group_id = ?::uuid
		order by usage_count desc, updated_at desc
	`, groupID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func filterAccountsByGroup(records []dao.ExpenseAccount, groupID string) []dao.ExpenseAccount {
	filtered := make([]dao.ExpenseAccount, 0, len(records))
	for _, record := range records {
		if record.GroupID == groupID {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func filterCategoriesByGroup(records []dao.ExpenseCategory, groupID string) []dao.ExpenseCategory {
	filtered := make([]dao.ExpenseCategory, 0, len(records))
	for _, record := range records {
		if record.GroupID == groupID {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func filterEntriesByGroup(records []dao.ExpenseEntry, groupID string) []dao.ExpenseEntry {
	filtered := make([]dao.ExpenseEntry, 0, len(records))
	for _, record := range records {
		if record.GroupID == groupID {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func filterAdjustmentsByGroup(records []dao.ExpenseCategoryAdjustment, groupID string) []dao.ExpenseCategoryAdjustment {
	filtered := make([]dao.ExpenseCategoryAdjustment, 0, len(records))
	for _, record := range records {
		if record.GroupID == groupID {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func filterMerchantsByGroup(records []dao.ExpenseMerchant, groupID string) []dao.ExpenseMerchant {
	filtered := make([]dao.ExpenseMerchant, 0, len(records))
	for _, record := range records {
		if record.GroupID == groupID {
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
	code := strings.ToUpper(strings.TrimSpace(value))
	if len(code) != 3 {
		return "USD"
	}
	for _, char := range code {
		if char < 'A' || char > 'Z' {
			return "USD"
		}
	}
	return code
}
