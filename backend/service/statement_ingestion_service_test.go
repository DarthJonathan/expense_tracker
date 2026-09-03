package service

import (
	"testing"

	"expense-tracker/backend/dao"
)

func TestBuildStatementValidationSeparatesPaymentsFromNewTransactions(t *testing.T) {
	date := "2026-08-01"
	previousBalance := 10000
	declaredNew := 2500
	grandTotal := 7500
	rows := []dao.ExpenseStatementIngestionRow{
		{SourceRowKey: "payment", OccurredOn: &date, Merchant: "PAYMENT", Amount: 5000, Currency: "SGD", Type: "income", StatementKind: "payment"},
		{SourceRowKey: "charge", OccurredOn: &date, Merchant: "SHOP", Amount: 3000, Currency: "SGD", Type: "expense", StatementKind: "transaction", Cardholder: "PRIMARY"},
		{SourceRowKey: "refund", OccurredOn: &date, Merchant: "REFUND", Amount: 500, Currency: "SGD", Type: "income", StatementKind: "refund", Cardholder: "PRIMARY"},
	}
	controls := []map[string]any{{"name": "PRIMARY", "declaredRowCount": 2, "declaredSubtotal": 2500}}

	validation := buildStatementValidation(rows, "SGD", &previousBalance, &declaredNew, &grandTotal, controls)
	assertValidationInt(t, validation, "signedTransactionActivity", -2500)
	assertValidationInt(t, validation, "signedNewTransactionActivity", 2500)
	assertValidationPass(t, validation, "newTransactionsTotal")
	assertValidationPass(t, validation, "balanceReconciliation")

	cardholders, ok := validation["cardholders"].([]map[string]any)
	if !ok || len(cardholders) != 1 {
		t.Fatalf("cardholder checks: %#v", validation["cardholders"])
	}
	if cardholders[0]["actualRowCount"] != 2 || cardholders[0]["actualSubtotal"] != 2500 {
		t.Fatalf("unexpected cardholder check: %#v", cardholders[0])
	}
}

func TestBuildStatementValidationKeepsForeignOriginalInPostedTotal(t *testing.T) {
	date := "2026-08-01"
	foreignAmount := 22360000
	declaredNew := 188713
	rows := []dao.ExpenseStatementIngestionRow{{
		SourceRowKey: "foreign", OccurredOn: &date, Merchant: "HOTEL", Amount: 188713,
		Currency: "SGD", ForeignAmount: &foreignAmount, ForeignCurrency: "JPY",
		Type: "expense", StatementKind: "transaction",
	}}

	validation := buildStatementValidation(rows, "SGD", nil, &declaredNew, nil, nil)
	assertValidationInt(t, validation, "signedNewTransactionActivity", 188713)
	assertValidationPass(t, validation, "newTransactionsTotal")
	foreignRows, ok := validation["foreignCurrencyRows"].([]string)
	if !ok || len(foreignRows) != 1 || foreignRows[0] != "foreign" {
		t.Fatalf("foreign rows: %#v", validation["foreignCurrencyRows"])
	}
}

func TestStatementIngestionStatusRequiresEveryDecision(t *testing.T) {
	rows := []dao.ExpenseStatementIngestionRow{{ID: "one", ReviewStatus: "new"}, {ID: "two", ReviewStatus: "unreviewed"}}
	if got := statementIngestionStatus(rows, "", dao.ExpenseStatementIngestionRow{}); got != statementStatusMatchingReview {
		t.Fatalf("status with unreviewed row = %q", got)
	}
	changed := rows[1]
	changed.ReviewStatus = "ignored"
	if got := statementIngestionStatus(rows, changed.ID, changed); got != statementStatusReady {
		t.Fatalf("status after all decisions = %q", got)
	}
}

func TestScoreStatementMatchUsesAmountDateAndMerchant(t *testing.T) {
	date := "2026-08-01"
	row := dao.ExpenseStatementIngestionRow{OccurredOn: &date, Merchant: "Example Shop Pte Ltd", Amount: 1234, Currency: "SGD", Type: "expense"}
	exact := statementMatchCandidate{OccurredOn: date, Merchant: "EXAMPLE SHOP", Amount: 1234, Currency: "SGD", Type: "expense"}
	if score := scoreStatementMatch(row, exact); score != 1 {
		t.Fatalf("exact score = %v, want 1", score)
	}
	wrongAmount := exact
	wrongAmount.Amount++
	if score := scoreStatementMatch(row, wrongAmount); score != 0 {
		t.Fatalf("wrong amount score = %v, want 0", score)
	}
	farDate := exact
	farDate.OccurredOn = "2026-08-05"
	if score := scoreStatementMatch(row, farDate); score != 0 {
		t.Fatalf("far date score = %v, want 0", score)
	}
}

func assertValidationInt(t *testing.T, validation map[string]any, key string, expected int) {
	t.Helper()
	if actual, ok := validation[key].(int); !ok || actual != expected {
		t.Fatalf("%s: got %#v, want %d", key, validation[key], expected)
	}
}

func assertValidationPass(t *testing.T, validation map[string]any, key string) {
	t.Helper()
	check, ok := validation[key].(map[string]any)
	if !ok || check["pass"] != true {
		t.Fatalf("%s: %#v", key, validation[key])
	}
}
