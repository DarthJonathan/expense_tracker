package database

import (
	"fmt"

	"expense-tracker/backend/dao"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	if err := db.Exec(`create extension if not exists pgcrypto`).Error; err != nil {
		return fmt.Errorf("create pgcrypto extension: %w", err)
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
	); err != nil {
		return fmt.Errorf("automigrate tables: %w", err)
	}

	indexes := []string{
		`alter table public.expense_categories
			add column if not exists type text not null default 'expense'`,
		`alter table public.expense_categories
			drop constraint if exists expense_categories_type_check`,
		`alter table public.expense_categories
			add constraint expense_categories_type_check check (type in ('expense', 'income'))`,
		`update public.expense_categories
			set type = 'income'
			where type = 'expense'
				and (
					lower(name) like '%income%'
					or lower(name) like '%salary%'
					or lower(name) like '%payroll%'
				)`,
		`create unique index if not exists expense_accounts_group_name_uidx
			on public.expense_accounts (group_id, lower(name))
			where deleted_at is null`,
		`create unique index if not exists expense_categories_group_name_uidx
			on public.expense_categories (group_id, lower(name))
			where deleted_at is null`,
		`create unique index if not exists expense_api_keys_hash_uidx
			on public.expense_api_keys (key_hash)
			where deleted_at is null and revoked_at is null`,
		`create index if not exists expense_api_keys_user_idx
			on public.expense_api_keys (user_id)
			where deleted_at is null and revoked_at is null`,
		`drop index if exists expense_merchants_group_normalized_name_uidx`,
		`create unique index if not exists expense_merchants_group_normalized_name_uidx
			on public.expense_merchants (group_id, normalized_name)`,
		`create index if not exists expense_entries_group_period_idx
			on public.expense_entries (group_id, occurred_on desc)
			where deleted_at is null`,
		`create index if not exists expense_entries_group_category_period_idx
			on public.expense_entries (group_id, category_id, occurred_on desc)
			where deleted_at is null`,
		`create index if not exists expense_entries_group_account_period_idx
			on public.expense_entries (group_id, account_id, occurred_on desc)
			where deleted_at is null`,
	}

	for _, statement := range indexes {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("create index: %w", err)
		}
	}

	return nil
}
