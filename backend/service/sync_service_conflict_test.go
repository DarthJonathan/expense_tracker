package service

import (
	"strings"
	"testing"

	"gorm.io/gorm/clause"
)

func TestEntryUpsertSQLRejectsStaleUpdates(t *testing.T) {
	table := "spendit.expense_entries"
	statement := entryUpsertSQL(table)
	expected := "where excluded.updated_at > spendit.expense_entries.updated_at"

	if !strings.Contains(statement, expected) {
		t.Fatalf("entry upsert is missing stale-write guard %q", expected)
	}
}

func TestNewerOnlyOnConflictRejectsStaleUpdates(t *testing.T) {
	table := "spendit.expense_accounts"
	conflict := newerOnlyOnConflict(table, map[string]any{"updated_at": "incoming"})
	if len(conflict.Where.Exprs) != 1 {
		t.Fatalf("expected one conflict guard, got %d", len(conflict.Where.Exprs))
	}

	expression, ok := conflict.Where.Exprs[0].(clause.Expr)
	if !ok {
		t.Fatalf("expected clause.Expr conflict guard, got %T", conflict.Where.Exprs[0])
	}

	expected := "excluded.updated_at > spendit.expense_accounts.updated_at"
	if expression.SQL != expected {
		t.Fatalf("unexpected conflict guard: got %q, want %q", expression.SQL, expected)
	}
}
