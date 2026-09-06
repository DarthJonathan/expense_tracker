package service

import (
	"math"
	"strings"
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

func TestValidateCombinedStatementRowsAcceptsTwoCompatibleRows(t *testing.T) {
	accountID := "account"
	rows := []dao.ExpenseStatementIngestionRow{
		{ID: "one", Currency: "SGD", Type: "expense", AccountID: &accountID},
		{ID: "two", Currency: "SGD", Type: "expense", AccountID: &accountID},
	}
	selected, err := normalizedCombinedStatementRowIDs([]string{"one", "two"})
	if err != nil {
		t.Fatalf("normalize selected rows: %v", err)
	}
	got, err := validateCombinedStatementRows(rows, selected, "master")
	if err != nil {
		t.Fatalf("validate combined rows: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("selected rows = %d, want 2", len(got))
	}
}

func TestValidateCombinedStatementRowsRejectsConflictAndIncompatibleRows(t *testing.T) {
	accountID, otherAccountID, masterID := "account", "other", "master"
	selected, err := normalizedCombinedStatementRowIDs([]string{"one", "two"})
	if err != nil {
		t.Fatalf("normalize selected rows: %v", err)
	}
	_, err = validateCombinedStatementRows([]dao.ExpenseStatementIngestionRow{
		{ID: "one", Currency: "SGD", Type: "expense", AccountID: &accountID},
		{ID: "two", Currency: "SGD", Type: "expense", AccountID: &otherAccountID},
	}, selected, masterID)
	if err == nil {
		t.Fatal("expected incompatible account error")
	}
	_, err = validateCombinedStatementRows([]dao.ExpenseStatementIngestionRow{
		{ID: "one", Currency: "SGD", Type: "expense", AccountID: &accountID},
		{ID: "two", Currency: "SGD", Type: "expense", AccountID: &accountID},
		{ID: "other", MatchExpenseID: &masterID},
	}, selected, masterID)
	if err == nil {
		t.Fatal("expected conflicting match error")
	}
}

func TestNormalizedCombinedStatementRowIDs(t *testing.T) {
	tests := []struct {
		name    string
		rowIDs  []string
		want    []string
		wantErr string
	}{
		{name: "trims ids", rowIDs: []string{" one ", "two"}, want: []string{"one", "two"}},
		{name: "requires two", rowIDs: []string{"one"}, wantErr: "at least two"},
		{name: "rejects duplicate", rowIDs: []string{"one", "one"}, wantErr: "duplicate"},
		{name: "rejects duplicate after trim", rowIDs: []string{"one", " one "}, wantErr: "duplicate"},
		{name: "rejects empty", rowIDs: []string{"one", "  "}, wantErr: "empty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizedCombinedStatementRowIDs(test.rowIDs)
			assertErrorContains(t, err, test.wantErr)
			if test.wantErr != "" {
				return
			}
			if len(got) != len(test.want) {
				t.Fatalf("normalized ids = %#v, want %#v", got, test.want)
			}
			for _, rowID := range test.want {
				if _, ok := got[rowID]; !ok {
					t.Fatalf("normalized ids missing %q: %#v", rowID, got)
				}
			}
		})
	}
}

func TestValidateCombinedStatementRowsRejectsInvalidSelections(t *testing.T) {
	accountID, otherAccountID, masterID, existingCombinedID := "account", "other", "master", "existing-combined"
	baseRows := []dao.ExpenseStatementIngestionRow{
		{ID: "one", Currency: "SGD", Type: "expense", AccountID: &accountID},
		{ID: "two", Currency: "SGD", Type: "expense", AccountID: &accountID},
	}
	tests := []struct {
		name    string
		mutate  func([]dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow
		wantErr string
	}{
		{name: "missing selected row", mutate: func(rows []dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow { return rows[:1] }, wantErr: "not found"},
		{name: "already combined", mutate: func(rows []dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow {
			rows[1].CombinedMatchID = &existingCombinedID
			return rows
		}, wantErr: "already part"},
		{name: "missing account", mutate: func(rows []dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow {
			rows[0].AccountID = nil
			rows[1].AccountID = nil
			return rows
		}, wantErr: "accountId"},
		{name: "different account", mutate: func(rows []dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow {
			rows[1].AccountID = &otherAccountID
			return rows
		}, wantErr: "same account"},
		{name: "different currency", mutate: func(rows []dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow {
			rows[1].Currency = "USD"
			return rows
		}, wantErr: "same currency"},
		{name: "different type", mutate: func(rows []dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow {
			rows[1].Type = "income"
			return rows
		}, wantErr: "same type"},
		{name: "master owned outside selection", mutate: func(rows []dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow {
			return append(rows, dao.ExpenseStatementIngestionRow{ID: "three", MatchExpenseID: &masterID})
		}, wantErr: "already assigned"},
	}
	selected := map[string]struct{}{"one": {}, "two": {}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := append([]dao.ExpenseStatementIngestionRow(nil), baseRows...)
			_, err := validateCombinedStatementRows(test.mutate(rows), selected, masterID)
			assertErrorContains(t, err, test.wantErr)
		})
	}
}

func TestValidateCombinedStatementRowsCanReplaceSelectedIndividualMatches(t *testing.T) {
	accountID, oldMasterOne, oldMasterTwo := "account", "old-one", "old-two"
	rows := []dao.ExpenseStatementIngestionRow{
		{ID: "one", Currency: "SGD", Type: "expense", AccountID: &accountID, ReviewStatus: "matched", MatchExpenseID: &oldMasterOne},
		{ID: "two", Currency: "SGD", Type: "expense", AccountID: &accountID, ReviewStatus: "matched", MatchExpenseID: &oldMasterTwo},
	}
	selected := map[string]struct{}{"one": {}, "two": {}}
	if _, err := validateCombinedStatementRows(rows, selected, "new-master"); err != nil {
		t.Fatalf("selected individual matches should be replaceable: %v", err)
	}
}

func TestValidateCombinedStatementGroupsAllowsOneMasterPerGroup(t *testing.T) {
	accountID, masterID, combinedID := "account", "master", "combined"
	rows := []dao.ExpenseStatementIngestionRow{
		{ID: "one", Currency: "SGD", Type: "expense", AccountID: &accountID, ReviewStatus: "matched", MatchExpenseID: &masterID, CombinedMatchID: &combinedID},
		{ID: "two", Currency: "SGD", Type: "expense", AccountID: &accountID, ReviewStatus: "matched", MatchExpenseID: &masterID, CombinedMatchID: &combinedID},
	}
	groups, err := validateCombinedStatementGroups(rows)
	if err != nil {
		t.Fatalf("validate combined group: %v", err)
	}
	if len(groups[combinedID]) != 2 {
		t.Fatalf("combined group rows = %d, want 2", len(groups[combinedID]))
	}
}

func TestValidateCombinedStatementGroupsRejectsOneRowAndSharedMaster(t *testing.T) {
	accountID, masterID, combinedID := "account", "master", "combined"
	_, err := validateCombinedStatementGroups([]dao.ExpenseStatementIngestionRow{{ID: "one", Currency: "SGD", Type: "expense", AccountID: &accountID, ReviewStatus: "matched", MatchExpenseID: &masterID, CombinedMatchID: &combinedID}})
	if err == nil {
		t.Fatal("expected one-row combined group error")
	}
	_, err = validateCombinedStatementGroups([]dao.ExpenseStatementIngestionRow{
		{ID: "one", Currency: "SGD", Type: "expense", AccountID: &accountID, ReviewStatus: "matched", MatchExpenseID: &masterID},
		{ID: "two", Currency: "SGD", Type: "expense", AccountID: &accountID, ReviewStatus: "matched", MatchExpenseID: &masterID, CombinedMatchID: &combinedID},
		{ID: "three", Currency: "SGD", Type: "expense", AccountID: &accountID, ReviewStatus: "matched", MatchExpenseID: &masterID, CombinedMatchID: &combinedID},
	})
	if err == nil {
		t.Fatal("expected shared master error")
	}
}

func TestValidateCombinedStatementGroupsRejectsMalformedGroups(t *testing.T) {
	accountID, otherAccountID := "account", "other"
	masterID, otherMasterID := "master", "other-master"
	combinedID := "combined"
	baseRows := []dao.ExpenseStatementIngestionRow{
		{ID: "one", Currency: "SGD", Type: "expense", AccountID: &accountID, ReviewStatus: "matched", MatchExpenseID: &masterID, CombinedMatchID: &combinedID},
		{ID: "two", Currency: "SGD", Type: "expense", AccountID: &accountID, ReviewStatus: "matched", MatchExpenseID: &masterID, CombinedMatchID: &combinedID},
	}
	tests := []struct {
		name    string
		mutate  func([]dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow
		wantErr string
	}{
		{name: "not matched", mutate: func(rows []dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow {
			rows[1].ReviewStatus = "unreviewed"
			return rows
		}, wantErr: "marked matched"},
		{name: "missing master", mutate: func(rows []dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow {
			rows[1].MatchExpenseID = nil
			return rows
		}, wantErr: "with a transaction"},
		{name: "different master", mutate: func(rows []dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow {
			rows[1].MatchExpenseID = &otherMasterID
			return rows
		}, wantErr: "same matched transaction"},
		{name: "missing account", mutate: func(rows []dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow {
			rows[0].AccountID = nil
			rows[1].AccountID = nil
			return rows
		}, wantErr: "accountId"},
		{name: "different account", mutate: func(rows []dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow {
			rows[1].AccountID = &otherAccountID
			return rows
		}, wantErr: "same currency, type, and account"},
		{name: "different currency", mutate: func(rows []dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow {
			rows[1].Currency = "USD"
			return rows
		}, wantErr: "same currency, type, and account"},
		{name: "different type", mutate: func(rows []dao.ExpenseStatementIngestionRow) []dao.ExpenseStatementIngestionRow {
			rows[1].Type = "income"
			return rows
		}, wantErr: "same currency, type, and account"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := append([]dao.ExpenseStatementIngestionRow(nil), baseRows...)
			_, err := validateCombinedStatementGroups(test.mutate(rows))
			assertErrorContains(t, err, test.wantErr)
		})
	}
}

func TestValidateCombinedStatementGroupsRejectsMasterUsedByTwoGroups(t *testing.T) {
	accountID, masterID, firstCombinedID, secondCombinedID := "account", "master", "combined-one", "combined-two"
	rows := []dao.ExpenseStatementIngestionRow{
		{ID: "one", Currency: "SGD", Type: "expense", AccountID: &accountID, ReviewStatus: "matched", MatchExpenseID: &masterID, CombinedMatchID: &firstCombinedID},
		{ID: "two", Currency: "SGD", Type: "expense", AccountID: &accountID, ReviewStatus: "matched", MatchExpenseID: &masterID, CombinedMatchID: &firstCombinedID},
		{ID: "three", Currency: "SGD", Type: "expense", AccountID: &accountID, ReviewStatus: "matched", MatchExpenseID: &masterID, CombinedMatchID: &secondCombinedID},
		{ID: "four", Currency: "SGD", Type: "expense", AccountID: &accountID, ReviewStatus: "matched", MatchExpenseID: &masterID, CombinedMatchID: &secondCombinedID},
	}
	_, err := validateCombinedStatementGroups(rows)
	assertErrorContains(t, err, "multiple rows or combined groups")
}

func TestEnsureCombinedMatchMasterCompatible(t *testing.T) {
	accountID := "account"
	rows := []dao.ExpenseStatementIngestionRow{
		{ID: "one", Currency: "SGD", Type: "expense", AccountID: &accountID},
		{ID: "two", Currency: "SGD", Type: "expense", AccountID: &accountID},
	}
	master := dao.ExpenseEntry{Currency: "SGD", Type: "expense", AccountID: accountID}
	if err := ensureCombinedMatchMasterCompatible(rows, master); err != nil {
		t.Fatalf("compatible master rejected: %v", err)
	}
	tests := []struct {
		name    string
		rows    []dao.ExpenseStatementIngestionRow
		master  dao.ExpenseEntry
		wantErr string
	}{
		{name: "requires two rows", rows: rows[:1], master: master, wantErr: "at least two"},
		{name: "currency", rows: rows, master: func() dao.ExpenseEntry { value := master; value.Currency = "USD"; return value }(), wantErr: "currency"},
		{name: "type", rows: rows, master: func() dao.ExpenseEntry { value := master; value.Type = "income"; return value }(), wantErr: "type"},
		{name: "account", rows: rows, master: func() dao.ExpenseEntry { value := master; value.AccountID = "other"; return value }(), wantErr: "account"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertErrorContains(t, ensureCombinedMatchMasterCompatible(test.rows, test.master), test.wantErr)
		})
	}
}

func TestBuildCombinedStatementConfirmationAggregatesAmountsAndProvenance(t *testing.T) {
	dateOne, dateTwo := "2026-08-01", "2026-08-02"
	foreignAmount := 10000
	ingestion := dao.ExpenseStatementIngestion{ID: "ingestion", SourceName: "statement.pdf", Institution: "DBS", SourceFingerprint: "fingerprint", StatementDate: &dateTwo}
	rows := []dao.ExpenseStatementIngestionRow{
		{ID: "one", SourceRowKey: "page-1-row-1", Amount: 1200, Currency: "USD", OccurredOn: &dateOne, StatementReference: "REF-1", ForeignAmount: &foreignAmount, ForeignCurrency: "JPY"},
		{ID: "two", SourceRowKey: "page-1-row-2", Amount: 800, Currency: "USD", OccurredOn: &dateTwo, StatementReference: "REF-2"},
	}
	rates := map[string]statementConfirmationRate{
		"one": {BaseAmount: 1500, FxRate: 1.25, FxRateDate: "2026-08-01"},
		"two": {BaseAmount: 1040, FxRate: 1.30, FxRateDate: "2026-08-02"},
	}
	got, err := buildCombinedStatementConfirmation(ingestion, "combined", rows, rates)
	if err != nil {
		t.Fatalf("build confirmation: %v", err)
	}
	if got.TotalAmount != 2000 || got.TotalBaseAmount != 2540 {
		t.Fatalf("aggregate amounts = %d/%d, want 2000/2540", got.TotalAmount, got.TotalBaseAmount)
	}
	if math.Abs(got.EffectiveFxRate-1.27) > 0.0000001 {
		t.Fatalf("effective FX rate = %v, want 1.27", got.EffectiveFxRate)
	}
	if got.FxRateDate != dateTwo {
		t.Fatalf("FX rate date = %q, want %q", got.FxRateDate, dateTwo)
	}
	if got.Provenance["ingestionId"] != ingestion.ID || got.Provenance["combinedMatchId"] != "combined" || got.Provenance["fxRateStrategy"] != "weighted_effective" {
		t.Fatalf("unexpected provenance identity: %#v", got.Provenance)
	}
	assertStringSlice(t, got.Provenance["sourceRowIds"], []string{"one", "two"})
	assertStringSlice(t, got.Provenance["sourceRowKeys"], []string{"page-1-row-1", "page-1-row-2"})
	sourceRows, ok := got.Provenance["sourceRows"].([]map[string]any)
	if !ok || len(sourceRows) != 2 {
		t.Fatalf("source rows = %#v", got.Provenance["sourceRows"])
	}
	if sourceRows[0]["foreignAmount"] != &foreignAmount || sourceRows[1]["fxRateDate"] != dateTwo {
		t.Fatalf("source row FX provenance = %#v", sourceRows)
	}
}

func TestBuildCombinedStatementConfirmationRejectsStaleOrInvalidTotals(t *testing.T) {
	rows := []dao.ExpenseStatementIngestionRow{{ID: "one", Amount: 100}, {ID: "two", Amount: 200}}
	validRates := map[string]statementConfirmationRate{
		"one": {BaseAmount: 100, FxRate: 1, FxRateDate: "2026-08-01"},
		"two": {BaseAmount: 200, FxRate: 1, FxRateDate: "2026-08-01"},
	}
	tests := []struct {
		name    string
		rows    []dao.ExpenseStatementIngestionRow
		rates   map[string]statementConfirmationRate
		wantErr string
	}{
		{name: "one row", rows: rows[:1], rates: validRates, wantErr: "at least two"},
		{name: "missing rate", rows: rows, rates: map[string]statementConfirmationRate{"one": validRates["one"]}, wantErr: "changed during confirmation"},
		{name: "zero posted total", rows: []dao.ExpenseStatementIngestionRow{{ID: "one", Amount: 0}, {ID: "two", Amount: 0}}, rates: validRates, wantErr: "greater than zero"},
		{name: "zero base total", rows: rows, rates: map[string]statementConfirmationRate{"one": {BaseAmount: 0, FxRate: 1, FxRateDate: "2026-08-01"}, "two": {BaseAmount: 0, FxRate: 1, FxRateDate: "2026-08-01"}}, wantErr: "greater than zero"},
		{name: "missing FX date", rows: rows, rates: map[string]statementConfirmationRate{"one": {BaseAmount: 100, FxRate: 1}, "two": {BaseAmount: 200, FxRate: 1}}, wantErr: "greater than zero"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := buildCombinedStatementConfirmation(dao.ExpenseStatementIngestion{}, "combined", test.rows, test.rates)
			assertErrorContains(t, err, test.wantErr)
		})
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

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want substring %q", err, want)
	}
}

func assertStringSlice(t *testing.T, actual any, expected []string) {
	t.Helper()
	values, ok := actual.([]string)
	if !ok || len(values) != len(expected) {
		t.Fatalf("slice = %#v, want %#v", actual, expected)
	}
	for index := range expected {
		if values[index] != expected[index] {
			t.Fatalf("slice = %#v, want %#v", values, expected)
		}
	}
}
