package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"expense-tracker/backend/dao"
	"expense-tracker/backend/request"
)

func TestSyncRecordAccessErrorRetainsForbiddenClassification(t *testing.T) {
	accessError := &SyncRecordAccessError{Collection: "categories", ID: "submitted-category"}
	wrapped := fmt.Errorf("prepare sync: %w", accessError)
	if !errors.Is(wrapped, ErrSyncRecordForbidden) {
		t.Fatal("record diagnostics must still produce HTTP 403, not an internal error")
	}
	var details *SyncRecordAccessError
	if !errors.As(wrapped, &details) || details.Collection != "categories" || details.ID != "submitted-category" {
		t.Fatal("record diagnostics were lost through error wrapping")
	}
	if !strings.Contains(wrapped.Error(), "categories record submitted-category") {
		t.Fatal("the API error must identify the submitted record for troubleshooting")
	}
}

func TestSyncConflictUsesServerVersionRegardlessOfClock(t *testing.T) {
	server := dao.ExpenseEntry{ID: "entry", Note: "server note", UpdatedAt: time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)}
	local := server
	local.Note = "local note"
	local.UpdatedAt = time.Date(2050, 1, 1, 0, 0, 0, 0, time.UTC)
	if !syncWriteConflicts("entries", local, server, "old-version") {
		t.Fatal("a fast device clock must not overwrite a changed server record")
	}
	version := syncRecordVersion(server)
	if syncWriteConflicts("entries", local, server, version) {
		t.Fatal("an edit based on the current server version should be accepted")
	}
	server.Note = "changed while popup was open"
	if !syncWriteConflicts("entries", local, server, version) {
		t.Fatal("a local conflict choice must be rechecked against the latest server version")
	}
	if !syncWriteConflicts("entries", local, nil, version) {
		t.Fatal("edit versus hard deletion must be a conflict")
	}
	if syncWriteConflicts("entries", local, nil, "missing") {
		t.Fatal("explicit recreation of a removed record should be accepted")
	}
}

func TestCanonicalFieldsDoNotCreateInitialSyncConflicts(t *testing.T) {
	server := dao.ExpenseEntry{ID: "entry", Amount: 1234, Currency: "SGD", BaseAmount: 1234, FxRate: 1, Metadata: dao.JSONMap{"fxMarkupPercent": 3.5}}
	local := server
	local.BaseAmount = 0
	local.FxRate = 0
	local.Metadata = nil
	local.UpdatedAt = time.Date(2050, 1, 1, 0, 0, 0, 0, time.UTC)
	if syncWriteConflicts("entries", local, server, "") {
		t.Fatal("server timestamps and computed FX fields should not cause a popup")
	}
	server.DeletedAt = &local.UpdatedAt
	if !syncWriteConflicts("entries", local, server, "") {
		t.Fatal("edit versus soft deletion must cause a popup")
	}
}

func TestStampSyncRecordPreservesServerIdentityAndUsesServerClock(t *testing.T) {
	creator := "server-user"
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	server := dao.ExpenseGroup{ID: "group", InviteCode: "SERVER", CreatedBy: &creator, CreatedAt: created}
	local := server
	local.Name = "Edited name"
	local.InviteCode = "LOCAL"
	local.CreatedAt = time.Date(2050, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	stamped := stampSyncRecord(local, server, now)
	if stamped.Name != local.Name || stamped.InviteCode != "SERVER" || !stamped.CreatedAt.Equal(created) || !stamped.UpdatedAt.Equal(now) {
		t.Fatalf("server fields were overwritten: %#v", stamped)
	}
}

func TestVersionCheckedWritesBypassDeviceTimestampGuard(t *testing.T) {
	if strings.Contains(entryUpsertSQL("spendit.expense_entries", true), "where excluded.updated_at >") {
		t.Fatal("a confirmed version-checked edit cannot be rejected by a device timestamp")
	}
	if len(newerOnlyOnConflict("spendit.expense_accounts", map[string]any{"name": "new"}, true).Where.Exprs) != 0 {
		t.Fatal("version-checked account edits still use a timestamp guard")
	}
}

func TestRecordLookupIsScopedAndLocked(t *testing.T) {
	query, args := syncReadQuery("select id::text as id from spendit.expense_entries where group_id = ?::uuid", []any{"group"}, []*request.SyncOptions{{RecordID: "entry", Limit: 1, Lock: true}})
	if !strings.Contains(query, "group_id = ?::uuid") || !strings.Contains(query, "and id = ?::uuid") || !strings.HasSuffix(query, "for update") || len(args) != 3 {
		t.Fatalf("version check lost its scope or row lock: %s %#v", query, args)
	}
}
