package service

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"expense-tracker/backend/dao"
	"expense-tracker/backend/request"
)

func TestSyncPagingKeepsScopeAndIncludesTombstones(t *testing.T) {
	query := "select * from expense_categories where group_id = ?::uuid and (scope = 'household' or owner_user_id = ?::uuid) order by updated_at desc"
	options := &request.SyncOptions{Mode: "pull", Collection: "categories", Cursor: "11111111-1111-4111-8111-111111111111", Limit: 100}
	got, args := syncReadQuery(query, []any{"group", "user"}, []*request.SyncOptions{options})
	if !strings.Contains(got, "group_id = ?::uuid and (scope = 'household' or owner_user_id = ?::uuid)") ||
		!strings.HasSuffix(got, "and id > ?::uuid order by id::uuid asc limit ?") {
		t.Fatalf("query lost scope or keyset pagination: %s", got)
	}
	if strings.Contains(got, "deleted_at is null") {
		t.Fatal("sync must include soft-deleted records")
	}
	if !reflect.DeepEqual(args, []any{"group", "user", options.Cursor, 101}) {
		t.Fatalf("unexpected args: %#v", args)
	}
	legacy, legacyArgs := syncReadQuery(query, []any{"group", "user"}, nil)
	if legacy != query || !reflect.DeepEqual(legacyArgs, []any{"group", "user"}) {
		t.Fatal("legacy sync query changed")
	}
}

func TestSyncPageContinuationDoesNotSkipLookaheadRow(t *testing.T) {
	options := &request.SyncOptions{Collection: "entries", Limit: 2}
	rows := []dao.ExpenseEntry{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	pageRows, page := trimSyncPage(rows, options, func(row dao.ExpenseEntry) string { return row.ID })
	if len(pageRows) != 2 || !page.HasMore || page.NextCursor != "b" {
		t.Fatalf("incorrect continuation: %#v, %#v", pageRows, page)
	}
	_, final := trimSyncPage(rows[2:], options, func(row dao.ExpenseEntry) string { return row.ID })
	if final.HasMore || final.NextCursor != "" {
		t.Fatalf("final page has continuation: %#v", final)
	}
	_, empty := trimSyncPage([]dao.ExpenseEntry{}, options, func(row dao.ExpenseEntry) string { return row.ID })
	if empty.HasMore {
		t.Fatal("empty page has continuation")
	}
}

func TestValidateSyncPagingOptions(t *testing.T) {
	valid := []*request.SyncOptions{nil, {Mode: "push"}, {Mode: "pull", Collection: "entries"}, {Mode: "pull", Collection: "categories", Limit: 200}}
	for _, options := range valid {
		if err := validateSyncOptions(options); err != nil {
			t.Fatalf("valid options rejected: %#v: %v", options, err)
		}
	}
	invalid := []*request.SyncOptions{{Mode: "unknown"}, {Mode: "pull", Collection: "users"}, {Mode: "pull", Collection: "entries", Cursor: "not-a-uuid"}, {Mode: "pull", Collection: "entries", Limit: 201}, {Mode: "pull", Collection: "entries", Limit: -1}}
	for _, options := range invalid {
		if err := validateSyncOptions(options); !errors.Is(err, ErrInvalidSyncOptions) {
			t.Fatalf("invalid options accepted: %#v: %v", options, err)
		}
	}
}

func TestSyncVersionsSkipUnchangedRowsWithoutSkippingPagination(t *testing.T) {
	options := &request.SyncOptions{Collection: "entries", Limit: 2}
	rows := []dao.ExpenseEntry{{ID: "a", Note: "original"}, {ID: "b"}, {ID: "c"}}
	_, initial := trimSyncPage(rows, options, func(row dao.ExpenseEntry) string { return row.ID })
	options.Known = initial.Versions
	unchanged, page := trimSyncPage(rows, options, func(row dao.ExpenseEntry) string { return row.ID })
	if len(unchanged) != 0 || !page.HasMore || page.NextCursor != "b" {
		t.Fatalf("unchanged page lost its cursor: %#v, %#v", unchanged, page)
	}
	// Canonical content can change without changing the client-provided timestamp.
	rows[0].Note = "canonical correction"
	changed, _ := trimSyncPage(rows, options, func(row dao.ExpenseEntry) string { return row.ID })
	if len(changed) != 1 || changed[0].ID != "a" {
		t.Fatalf("content change was missed: %#v", changed)
	}
}
