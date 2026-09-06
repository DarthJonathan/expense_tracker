package service

import (
	"reflect"
	"testing"
	"time"

	"expense-tracker/backend/dao"
	"expense-tracker/backend/request"
)

func TestCombinedStatementActionKeepsLegacyMatchingContract(t *testing.T) {
	draft, err := normalizeCombinedStatementDraft(&request.CreateCombinedStatementMatchRequest{MatchExpenseID: "existing-master"})
	if err != nil || draft != nil {
		t.Fatalf("legacy match: draft=%+v error=%v", draft, err)
	}
}

func TestCombinedStatementNewActionValidation(t *testing.T) {
	valid := request.CombinedStatementTransactionRequest{Merchant: " Taobao ", OccurredOn: "2026-07-15", CategoryID: "shopping", Note: " Two charges "}
	tests := []struct {
		name   string
		change func(*request.CreateCombinedStatementMatchRequest)
		want   string
	}{
		{"valid draft", func(r *request.CreateCombinedStatementMatchRequest) {}, ""},
		{"both actions", func(r *request.CreateCombinedStatementMatchRequest) { r.MatchExpenseID = "master" }, "either"},
		{"neither action", func(r *request.CreateCombinedStatementMatchRequest) { r.NewTransaction = nil }, "required"},
		{"empty merchant", func(r *request.CreateCombinedStatementMatchRequest) { r.NewTransaction.Merchant = " " }, "merchant"},
		{"empty date", func(r *request.CreateCombinedStatementMatchRequest) { r.NewTransaction.OccurredOn = "" }, "occurredOn"},
		{"invalid date", func(r *request.CreateCombinedStatementMatchRequest) { r.NewTransaction.OccurredOn = "2026-02-30" }, "occurredOn"},
		{"empty category", func(r *request.CreateCombinedStatementMatchRequest) { r.NewTransaction.CategoryID = " " }, "categoryId"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := valid
			req := request.CreateCombinedStatementMatchRequest{NewTransaction: &input}
			tt.change(&req)
			draft, err := normalizeCombinedStatementDraft(&req)
			assertErrorContains(t, err, tt.want)
			if tt.want == "" && (draft.Merchant != "Taobao" || draft.Note != "Two charges") {
				t.Fatalf("draft not normalized: %+v", draft)
			}
		})
	}
}

func combinedNewRows() []dao.ExpenseStatementIngestionRow {
	accountID, combinedID, date := "account", "combined-new", "2026-07-15"
	draft := dao.CombinedStatementTransaction{Merchant: "Home purchases", OccurredOn: "2026-07-16", CategoryID: "shopping", Note: "Two charges"}
	return []dao.ExpenseStatementIngestionRow{
		{ID: "one", SourceRowKey: "page1-1", AccountID: &accountID, CombinedMatchID: &combinedID, CombinedTransaction: &draft, ReviewStatus: "new", Merchant: "TAOBAO first charge", OccurredOn: &date, Amount: 16092, Currency: "SGD", Type: "expense"},
		{ID: "two", SourceRowKey: "page1-2", AccountID: &accountID, CombinedMatchID: &combinedID, CombinedTransaction: &draft, ReviewStatus: "new", Merchant: "TAOBAO second charge", OccurredOn: &date, Amount: 65352, Currency: "SGD", Type: "expense"},
	}
}

func TestNewCombinedSelectionAllowsUnmatchedUnselectedRows(t *testing.T) {
	rows := combinedNewRows()
	for i := range rows {
		rows[i].CombinedMatchID = nil
		rows[i].CombinedTransaction = nil
	}
	rows = append(rows, dao.ExpenseStatementIngestionRow{ID: "outside", ReviewStatus: "unreviewed"})
	selected, err := validateCombinedStatementRows(rows, map[string]struct{}{"one": {}, "two": {}}, "")
	if err != nil || len(selected) != 2 {
		t.Fatalf("new selection: %v, %v", selected, err)
	}
}

func TestCombinedNewGroupRejectsInconsistentDraftsAndMixedActions(t *testing.T) {
	tests := []struct {
		name   string
		change func([]dao.ExpenseStatementIngestionRow)
	}{
		{"missing draft", func(rows []dao.ExpenseStatementIngestionRow) { rows[1].CombinedTransaction = nil }},
		{"master assigned to new", func(rows []dao.ExpenseStatementIngestionRow) { rows[1].MatchExpenseID = stringPointer("master") }},
		{"different draft", func(rows []dao.ExpenseStatementIngestionRow) {
			copy := *rows[1].CombinedTransaction
			copy.Merchant = "Changed"
			rows[1].CombinedTransaction = &copy
		}},
		{"mixed actions", func(rows []dao.ExpenseStatementIngestionRow) {
			rows[1].ReviewStatus = "matched"
			rows[1].CombinedTransaction = nil
			rows[1].MatchExpenseID = stringPointer("master")
		}},
		{"invalid draft", func(rows []dao.ExpenseStatementIngestionRow) { rows[1].CombinedTransaction.OccurredOn = "invalid" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := combinedNewRows()
			tt.change(rows)
			if _, err := validateCombinedStatementGroups(rows); err == nil {
				t.Fatal("invalid group accepted")
			}
		})
	}
}

func TestMultipleCombinedNewGroupsCanCoexistWithOrdinaryMatches(t *testing.T) {
	rows := combinedNewRows()
	other := combinedNewRows()
	for i := range other {
		other[i].ID += "-other"
		other[i].CombinedMatchID = stringPointer("other-group")
	}
	rows = append(rows, other...)
	rows = append(rows, dao.ExpenseStatementIngestionRow{ID: "ordinary", ReviewStatus: "matched", MatchExpenseID: stringPointer("master")})
	groups, err := validateCombinedStatementGroups(rows)
	if err != nil || len(groups) != 2 {
		t.Fatalf("independent new groups rejected: %v %v", groups, err)
	}
}

func TestCombinedNewEntrySumsSourcesAndPreservesOriginalFields(t *testing.T) {
	rows := combinedNewRows()
	before := append([]dao.ExpenseStatementIngestionRow(nil), rows...)
	now := time.Now().UTC()
	rates := map[string]statementConfirmationRate{"one": {BaseAmount: 16092, FxRate: 1, FxRateDate: "2026-07-15"}, "two": {BaseAmount: 65352, FxRate: 1, FxRateDate: "2026-07-15"}}
	entry, err := buildCombinedStatementEntry(dao.ExpenseStatementIngestion{ID: "statement"}, "group", "user", "new-master", "combined-new", rows, rates, "SGD", now)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Amount != 81444 || entry.BaseAmount != 81444 || entry.FxRate != 1 {
		t.Fatalf("incorrect sum: %+v", entry)
	}
	if entry.Merchant != "Home purchases" || entry.OccurredOn != "2026-07-16" || entry.CategoryID != "shopping" || entry.Note != "Two charges" {
		t.Fatalf("draft fields not used: %+v", entry)
	}
	if entry.ID != "new-master" || entry.GroupID != "group" || entry.AccountID != "account" || derefString(entry.CreatedBy) != "user" {
		t.Fatalf("incorrect ownership: %+v", entry)
	}
	if !reflect.DeepEqual(rows, before) {
		t.Fatal("source rows mutated")
	}
	provenance := entry.Metadata["statementIngestion"].(map[string]any)
	assertStringSlice(t, provenance["sourceRowIds"], []string{"one", "two"})
	sources := provenance["sourceRows"].([]map[string]any)
	if sources[0]["merchant"] != "TAOBAO first charge" || derefString(sources[0]["occurredOn"].(*string)) != "2026-07-15" {
		t.Fatalf("source provenance lost: %+v", sources)
	}
}
