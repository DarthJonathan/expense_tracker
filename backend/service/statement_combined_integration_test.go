package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"expense-tracker/backend/dao"
	"expense-tracker/backend/database"
	"expense-tracker/backend/request"

	uuid "github.com/hashicorp/go-uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Run only against a designated test database. All fixtures live in a uniquely
// named schema, which is removed at the end; no application schema is touched.
func TestCombinedStatementPersistenceAndConfirmation(t *testing.T) {
	dsn := os.Getenv("STATEMENT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set STATEMENT_TEST_DATABASE_URL for PostgreSQL lifecycle tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	newID := func() string {
		t.Helper()
		id, err := uuid.GenerateUUID()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	schema := "statement_test_" + strings.ReplaceAll(newID(), "-", "")
	previousSchema := dao.Schema()
	if err := dao.SetSchema(schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Exec("drop schema if exists " + schema + " cascade").Error
		_ = dao.SetSchema(previousSchema)
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	s := &ExpenseService{DB: db}
	group := dao.ExpenseGroup{ID: newID(), Name: "Test", InviteCode: newID()}
	user := dao.ExpenseUser{ID: newID(), Email: "test@example.test", PasswordHash: "fixture", DisplayName: "Test", GroupID: &group.ID, BaseCurrency: "SGD"}
	account := dao.ExpenseAccount{ID: newID(), GroupID: group.ID, Name: "Card", Type: "card"}
	category := dao.ExpenseCategory{ID: newID(), GroupID: group.ID, Name: "Shopping", Type: "expense", Scope: "household"}
	for _, record := range []any{&group, &user, &account, &category} {
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	seedStatement := func() (string, []string) {
		t.Helper()
		ingestion := dao.ExpenseStatementIngestion{ID: newID(), GroupID: group.ID, AccountID: account.ID, SourceName: "sample.pdf", Status: statementStatusMatchingReview, StatementCurrency: "SGD", ParsedRowCount: 2}
		if err := db.Create(&ingestion).Error; err != nil {
			t.Fatal(err)
		}
		ids := []string{newID(), newID()}
		date := "2026-07-15"
		for i, amount := range []int{16092, 65352} {
			row := dao.ExpenseStatementIngestionRow{ID: ids[i], IngestionID: ingestion.ID, GroupID: group.ID, AccountID: &account.ID, SourceRowKey: fmt.Sprintf("page1-%d", i), Merchant: fmt.Sprintf("Source merchant %d", i), OccurredOn: &date, Amount: amount, Currency: "SGD", Type: "expense", ReviewStatus: "unreviewed"}
			if err := db.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
		}
		return ingestion.ID, ids
	}
	countEntries := func() int64 {
		t.Helper()
		var count int64
		if err := db.Model(&dao.ExpenseEntry{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		return count
	}
	statementID, ids := seedStatement()
	// Simulate an installed schema with source rows but without the new column.
	if err := db.Exec("alter table " + dao.QualifiedTable("expense_statement_ingestion_rows") + " drop column combined_transaction").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	old, err := s.getStatementIngestion(ctx, group.ID, statementID, false)
	if err != nil || len(old.Rows) != 2 {
		t.Fatalf("legacy rows after migration: %v %v", old, err)
	}
	if old.Rows[0].CombinedTransaction != nil || old.Rows[0].Amount != 16092 {
		t.Fatal("migration changed existing source data")
	}
	draft := &request.CombinedStatementTransactionRequest{Merchant: "Combined purchase", OccurredOn: "2026-07-16", CategoryID: category.ID, Note: "Reviewed note"}
	req := &request.CreateCombinedStatementMatchRequest{RowIDs: ids, NewTransaction: draft}
	saved, err := s.CreateCombinedStatementMatch(ctx, group.ID, statementID, user.ID, req)
	if err != nil {
		t.Fatalf("stage new: %v", err)
	}
	if countEntries() != 0 || saved.Ingestion.Status != statementStatusReady {
		t.Fatal("staging wrote a master transaction or did not become ready")
	}
	reloaded, err := s.getStatementIngestion(ctx, group.ID, statementID, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range reloaded.Rows {
		if row.CombinedTransaction == nil || row.CombinedTransaction.Merchant != draft.Merchant || row.Merchant == draft.Merchant || row.MatchExpenseID != nil {
			t.Fatalf("draft/source persistence: %+v", row)
		}
	}
	combinedID := derefString(reloaded.Rows[0].CombinedMatchID)
	uncombined, err := s.DissolveCombinedStatementMatch(ctx, group.ID, statementID, combinedID, user.ID)
	if err != nil {
		t.Fatalf("uncombine new: %v", err)
	}
	for _, row := range uncombined.Rows {
		if row.CombinedTransaction != nil || row.CombinedMatchID != nil || row.ReviewStatus != "unreviewed" {
			t.Fatalf("uncombine retained draft: %+v", row)
		}
	}
	if countEntries() != 0 {
		t.Fatal("uncombine created a transaction")
	}
	if _, err := s.CreateCombinedStatementMatch(ctx, group.ID, statementID, user.ID, req); err != nil {
		t.Fatal(err)
	}
	confirmed, err := s.ConfirmStatementIngestion(ctx, group.ID, statementID, user.ID)
	if err != nil {
		t.Fatalf("confirm new: %v", err)
	}
	if countEntries() != 1 || confirmed.Ingestion.Status != statementStatusConfirmed {
		t.Fatal("combined group did not produce exactly one transaction")
	}
	masterID := derefString(confirmed.Rows[0].ConfirmedExpenseID)
	for _, row := range confirmed.Rows {
		if derefString(row.ConfirmedExpenseID) != masterID || masterID == "" {
			t.Fatal("source rows not linked to same new master")
		}
	}
	var entry dao.ExpenseEntry
	if err := db.Where("id = ?::uuid", masterID).First(&entry).Error; err != nil {
		t.Fatal(err)
	}
	if entry.Amount != 81444 || entry.Merchant != draft.Merchant || entry.CategoryID != category.ID || entry.Note != draft.Note || !strings.HasPrefix(entry.OccurredOn, draft.OccurredOn) {
		t.Fatalf("incorrect new entry: %+v", entry)
	}
	provenance := entry.Metadata["statementIngestion"].(map[string]any)
	if len(provenance["sourceRowIds"].([]any)) != 2 {
		t.Fatal("missing source provenance")
	}
	if _, err := s.ConfirmStatementIngestion(ctx, group.ID, statementID, user.ID); err != nil {
		t.Fatal(err)
	}
	if countEntries() != 1 {
		t.Fatal("repeat confirmation created a duplicate")
	}
	if _, err := s.DissolveCombinedStatementMatch(ctx, group.ID, statementID, derefString(confirmed.Rows[0].CombinedMatchID), user.ID); err == nil {
		t.Fatal("confirmed group allowed editing")
	}

	// The legacy match-only payload must still update exactly one master while
	// preserving its established merchant, date, category, note and other metadata.
	legacyStatement, legacyRows := seedStatement()
	if err := db.Model(&entry).Updates(map[string]any{"amount": 80000, "metadata": gorm.Expr("?::jsonb", `{"existing":"keep"}`)}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateCombinedStatementMatch(ctx, group.ID, legacyStatement, user.ID, &request.CreateCombinedStatementMatchRequest{RowIDs: legacyRows, MatchExpenseID: masterID}); err != nil {
		t.Fatalf("legacy match staging: %v", err)
	}
	if _, err := s.ConfirmStatementIngestion(ctx, group.ID, legacyStatement, user.ID); err != nil {
		t.Fatalf("legacy match confirmation: %v", err)
	}
	if err := db.Where("id = ?::uuid", masterID).First(&entry).Error; err != nil {
		t.Fatal(err)
	}
	if countEntries() != 1 || entry.Amount != 81444 || entry.Merchant != draft.Merchant || entry.Note != draft.Note || entry.Metadata["existing"] != "keep" {
		t.Fatalf("legacy match changed unrelated fields: %+v", entry)
	}

	// Revalidate references at confirmation, including a category deleted after
	// the draft was saved. Failure must leave the ingestion and ledger untouched.
	invalidStatement, invalidRows := seedStatement()
	if _, err := s.CreateCombinedStatementMatch(ctx, group.ID, invalidStatement, user.ID, &request.CreateCombinedStatementMatchRequest{RowIDs: invalidRows, NewTransaction: draft}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&category).Update("deleted_at", time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmStatementIngestion(ctx, group.ID, invalidStatement, user.ID); err == nil {
		t.Fatal("deleted category accepted")
	}
	if countEntries() != 1 {
		t.Fatal("failed confirmation changed ledger")
	}
	failed, _ := s.getStatementIngestion(ctx, group.ID, invalidStatement, false)
	if failed.Ingestion.Status != statementStatusReady || failed.Rows[0].ConfirmedExpenseID != nil {
		t.Fatal("failed confirmation changed staging")
	}
	if _, err := s.DeleteStatementIngestion(ctx, group.ID, statementID, user.ID); err != nil {
		t.Fatal(err)
	}
	if countEntries() != 1 {
		t.Fatal("staging deletion removed master transaction")
	}
}
