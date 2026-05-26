package database

import (
	"fmt"
	"strings"

	"expense-tracker/backend/dao"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	if err := db.Exec(`create extension if not exists pgcrypto`).Error; err != nil {
		return fmt.Errorf("create pgcrypto extension: %w", err)
	}
	if err := db.Exec(`create extension if not exists pg_trgm`).Error; err != nil {
		return fmt.Errorf("create pg_trgm extension: %w", err)
	}
	if err := db.Exec(`create extension if not exists unaccent`).Error; err != nil {
		return fmt.Errorf("create unaccent extension: %w", err)
	}
	schema := dao.Schema()
	if err := db.Exec(fmt.Sprintf("create schema if not exists %s", quoteIdentifier(schema))).Error; err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	if err := db.AutoMigrate(
		&dao.ExpenseUser{},
		&dao.ExpenseAPIKey{},
		&dao.ExpenseGroup{},
		&dao.ExpenseAccount{},
		&dao.ExpenseCategory{},
		&dao.ExpenseEntry{},
		&dao.ExpenseCategoryAdjustment{},
		&dao.ExpenseMerchant{},
		&dao.ExpenseMerchantCategoryMap{},
		&dao.ExpenseCategoryRule{},
	); err != nil {
		return fmt.Errorf("automigrate tables: %w", err)
	}

	tables := map[string]string{
		"categories":           dao.QualifiedTable("expense_categories"),
		"accounts":             dao.QualifiedTable("expense_accounts"),
		"apiKeys":              dao.QualifiedTable("expense_api_keys"),
		"merchants":            dao.QualifiedTable("expense_merchants"),
		"merchantCategoryMaps": dao.QualifiedTable("expense_merchant_category_maps"),
		"categoryRules":        dao.QualifiedTable("expense_category_rules"),
		"entries":              dao.QualifiedTable("expense_entries"),
	}

	indexes := []string{
		fmt.Sprintf(`alter table %s
			add column if not exists group_id uuid references %s(id) on delete set null`,
			dao.QualifiedTable("expense_users"),
			dao.QualifiedTable("expense_groups")),
		fmt.Sprintf(`update %s u
			set group_id = g.id
			from (
				select distinct on (created_by) created_by, id
				from %s
				where created_by is not null and deleted_at is null
				order by created_by, updated_at desc
			) g
			where u.group_id is null and u.id = g.created_by`,
			dao.QualifiedTable("expense_users"),
			dao.QualifiedTable("expense_groups")),
		fmt.Sprintf(`alter table %s
			add column if not exists type text not null default 'expense'`,
			tables["categories"]),
		fmt.Sprintf(`alter table %s
			add column if not exists scope text not null default 'household'`,
			tables["categories"]),
		fmt.Sprintf(`alter table %s
			add column if not exists owner_user_id uuid`,
			tables["categories"]),
		fmt.Sprintf(`alter table %s
			drop constraint if exists expense_categories_type_check`,
			tables["categories"]),
		fmt.Sprintf(`alter table %s
			add constraint expense_categories_type_check check (type in ('expense', 'income'))`,
			tables["categories"]),
		fmt.Sprintf(`alter table %s
			drop constraint if exists expense_categories_scope_check`,
			tables["categories"]),
		fmt.Sprintf(`alter table %s
			add constraint expense_categories_scope_check check (scope in ('household', 'user'))`,
			tables["categories"]),
		fmt.Sprintf(`alter table %s
			drop constraint if exists expense_categories_scope_owner_check`,
			tables["categories"]),
		fmt.Sprintf(`alter table %s
			add constraint expense_categories_scope_owner_check
			check (
				(scope = 'household' and owner_user_id is null) or
				(scope = 'user' and owner_user_id is not null)
			)`,
			tables["categories"]),
		`update ` + tables["categories"] + `
			set type = 'income'
			where type = 'expense'
				and (
					lower(name) like '%income%'
					or lower(name) like '%salary%'
					or lower(name) like '%payroll%'
				)`,
		`update ` + tables["categories"] + `
			set scope = 'household'
			where coalesce(scope, '') = ''`,
		fmt.Sprintf(`create unique index if not exists expense_accounts_group_name_uidx
			on %s (group_id, lower(name))
			where deleted_at is null`,
			tables["accounts"]),
		fmt.Sprintf(
			"drop index if exists %s.%s",
			quoteIdentifier(schema),
			quoteIdentifier("expense_categories_group_name_uidx"),
		),
		fmt.Sprintf(`create unique index if not exists expense_categories_group_name_uidx
			on %s (group_id, scope, coalesce(owner_user_id, '00000000-0000-0000-0000-000000000000'::uuid), lower(name))
			where deleted_at is null`,
			tables["categories"]),
		fmt.Sprintf(`create unique index if not exists expense_api_keys_hash_uidx
			on %s (key_hash)
			where deleted_at is null and revoked_at is null`,
			tables["apiKeys"]),
		fmt.Sprintf(`create index if not exists expense_api_keys_user_idx
			on %s (user_id)
			where deleted_at is null and revoked_at is null`,
			tables["apiKeys"]),
		fmt.Sprintf(
			"drop index if exists %s.%s",
			quoteIdentifier(schema),
			quoteIdentifier("expense_merchants_group_normalized_name_uidx"),
		),
		fmt.Sprintf(`create unique index if not exists expense_merchants_group_normalized_name_uidx
			on %s (group_id, normalized_name)`,
			tables["merchants"]),
		fmt.Sprintf(`create unique index if not exists expense_merchant_category_maps_group_merchant_type_uidx
			on %s (group_id, normalized_merchant, entry_type)
			where deleted_at is null`,
			tables["merchantCategoryMaps"]),
		fmt.Sprintf(`create index if not exists expense_merchant_category_maps_merchant_trgm_idx
			on %s using gin (normalized_merchant gin_trgm_ops)`,
			tables["merchantCategoryMaps"]),
		fmt.Sprintf(`create index if not exists expense_category_rules_group_priority_idx
			on %s (group_id, priority, updated_at desc)
			where deleted_at is null and enabled = true`,
			tables["categoryRules"]),
		fmt.Sprintf(`alter table %s
			drop constraint if exists expense_merchant_category_maps_entry_type_check`,
			tables["merchantCategoryMaps"]),
		fmt.Sprintf(`alter table %s
			add constraint expense_merchant_category_maps_entry_type_check check (entry_type in ('expense', 'income'))`,
			tables["merchantCategoryMaps"]),
		fmt.Sprintf(`alter table %s
			drop constraint if exists expense_category_rules_entry_type_check`,
			tables["categoryRules"]),
		fmt.Sprintf(`alter table %s
			add constraint expense_category_rules_entry_type_check check (entry_type in ('expense', 'income', 'any'))`,
			tables["categoryRules"]),
		fmt.Sprintf(`alter table %s
			drop constraint if exists expense_category_rules_match_field_check`,
			tables["categoryRules"]),
		fmt.Sprintf(`alter table %s
			add constraint expense_category_rules_match_field_check check (match_field in ('merchant', 'note', 'account_type'))`,
			tables["categoryRules"]),
		fmt.Sprintf(`alter table %s
			drop constraint if exists expense_category_rules_match_kind_check`,
			tables["categoryRules"]),
		fmt.Sprintf(`alter table %s
			add constraint expense_category_rules_match_kind_check check (match_kind in ('contains', 'prefix', 'equals', 'regex'))`,
			tables["categoryRules"]),
		fmt.Sprintf(`create index if not exists expense_entries_group_period_idx
			on %s (group_id, occurred_on desc)
			where deleted_at is null`,
			tables["entries"]),
		fmt.Sprintf(`create index if not exists expense_entries_group_category_period_idx
			on %s (group_id, category_id, occurred_on desc)
			where deleted_at is null`,
			tables["entries"]),
		fmt.Sprintf(`create index if not exists expense_entries_group_account_period_idx
			on %s (group_id, account_id, occurred_on desc)
			where deleted_at is null`,
			tables["entries"]),
		fmt.Sprintf(`alter table %s
			add column if not exists metadata jsonb not null default '{}'::jsonb`,
			tables["entries"]),
		`update ` + tables["entries"] + `
			set currency = 'SGD'
			where coalesce(currency, '') <> 'SGD'`,
	}

	for _, statement := range indexes {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("create index: %w", err)
		}
	}

	return nil
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
