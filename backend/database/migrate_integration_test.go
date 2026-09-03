package database

import (
	"os"
	"testing"

	"expense-tracker/backend/dao"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMigrateAddsStatementTablesWithoutChangingExistingExpenses(t *testing.T) {
	databaseURL := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set MIGRATION_TEST_DATABASE_URL to run the PostgreSQL migration test")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("open migration database: %v", err)
	}

	const schema = "spendit_migration_test"
	if err := db.Exec(`drop schema if exists spendit_migration_test cascade`).Error; err != nil {
		t.Fatalf("reset test schema: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Exec(`drop schema if exists spendit_migration_test cascade`).Error
		_ = dao.SetSchema("spendit")
	})
	if err := dao.SetSchema(schema); err != nil {
		t.Fatalf("set test schema: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("initial migration: %v", err)
	}

	groupID := "11111111-1111-1111-1111-111111111111"
	userID := "22222222-2222-2222-2222-222222222222"
	accountID := "33333333-3333-3333-3333-333333333333"
	categoryID := "44444444-4444-4444-4444-444444444444"
	entryID := "55555555-5555-5555-5555-555555555555"
	statements := []string{
		`insert into spendit_migration_test.expense_groups (id, name, invite_code) values ('` + groupID + `', 'Existing group', 'EXIST1')`,
		`insert into spendit_migration_test.expense_users (id, email, password_hash, display_name, group_id) values ('` + userID + `', 'existing@example.test', 'hash', 'Existing user', '` + groupID + `')`,
		`insert into spendit_migration_test.expense_accounts (id, group_id, name, type) values ('` + accountID + `', '` + groupID + `', 'Existing card', 'card')`,
		`insert into spendit_migration_test.expense_categories (id, group_id, name, type, scope) values ('` + categoryID + `', '` + groupID + `', 'Existing category', 'expense', 'household')`,
		`insert into spendit_migration_test.expense_entries (id, group_id, account_id, category_id, type, amount, currency, base_amount, base_currency, fx_rate, fx_rate_date, occurred_on, merchant, metadata) values ('` + entryID + `', '` + groupID + `', '` + accountID + `', '` + categoryID + `', 'expense', 1234, 'SGD', 1234, 'SGD', 1, '2026-08-01', '2026-08-01', 'Existing merchant', '{"existing":"preserved"}')`,
		`drop table spendit_migration_test.expense_statement_ingestion_rows`,
		`drop table spendit_migration_test.expense_statement_ingestions`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("prepare existing schema: %v", err)
		}
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("upgrade migration: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("idempotent migration: %v", err)
	}

	var count int64
	if err := db.Raw(`select count(*) from spendit_migration_test.expense_entries where id = ?::uuid and amount = 1234 and metadata->>'existing' = 'preserved'`, entryID).Scan(&count).Error; err != nil {
		t.Fatalf("verify existing expense: %v", err)
	}
	if count != 1 {
		t.Fatalf("existing expense was changed or removed")
	}
	var statementTableCount int64
	if err := db.Raw(`select count(*) from information_schema.tables where table_schema = ? and table_name in ('expense_statement_ingestions', 'expense_statement_ingestion_rows')`, schema).Scan(&statementTableCount).Error; err != nil {
		t.Fatalf("verify statement tables: %v", err)
	}
	if statementTableCount != 2 {
		t.Fatalf("statement tables created = %d, want 2", statementTableCount)
	}
}
