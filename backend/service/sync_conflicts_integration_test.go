package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"expense-tracker/backend/dao"
	"expense-tracker/backend/database"
	"expense-tracker/backend/request"
	"expense-tracker/backend/response"

	uuid "github.com/hashicorp/go-uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSyncServerAuthorityPostgres(t *testing.T) {
	dsn := os.Getenv("SYNC_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SYNC_TEST_DATABASE_URL to run PostgreSQL sync integration checks")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	newID := func() string {
		id, err := uuid.GenerateUUID()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	schema := "sync_test_" + strings.ReplaceAll(newID(), "-", "")
	previousSchema := dao.Schema()
	if err := dao.SetSchema(schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Exec("drop schema if exists " + schema + " cascade").Error
		_ = dao.SetSchema(previousSchema)
	})
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	groupID, userID, accountID, categoryID, entryID := newID(), newID(), newID(), newID(), newID()
	fixtures := []any{
		&dao.ExpenseGroup{ID: groupID, Name: "Sync test", InviteCode: "SYNC_TEST"},
		&dao.ExpenseUser{ID: userID, Email: "sync@example.test", PasswordHash: "test", DisplayName: "Sync test", GroupID: &groupID, BaseCurrency: "SGD"},
		&dao.ExpenseAccount{ID: accountID, GroupID: groupID, Name: "Test cash", Type: "cash"},
		&dao.ExpenseCategory{ID: categoryID, GroupID: groupID, Name: "Test food", Type: "expense", Scope: "household"},
	}
	for _, fixture := range fixtures {
		if err := db.Create(fixture).Error; err != nil {
			t.Fatal(err)
		}
	}
	service := NewSyncService(db)
	entry := dao.ExpenseEntry{ID: entryID, GroupID: groupID, AccountID: accountID, CategoryID: categoryID, Type: "expense", Amount: 1200, Currency: "SGD", BaseAmount: 1200, BaseCurrency: "SGD", FxRate: 1, FxRateDate: "2026-10-01", OccurredOn: "2026-10-01", Merchant: "Test merchant", Note: "initial", Metadata: dao.JSONMap{}}
	makeRequest := func(row dao.ExpenseEntry, base string) *request.SyncRequest {
		return &request.SyncRequest{Settings: request.SyncSettingsRequest{ActiveGroupID: groupID, DeviceUserID: userID, BaseCurrency: "USD"},
			Entries: []dao.ExpenseEntry{row}, Sync: &request.SyncOptions{Mode: "push", Version: 3, BaseVersions: map[string]map[string]string{"entries": {entryID: base}}}}
	}
	initial, err := service.Sync(context.Background(), userID, makeRequest(entry, ""))
	if err != nil {
		t.Fatal(err)
	}
	if initial.Settings.BaseCurrency != "SGD" || len(initial.Entries) != 1 {
		t.Fatalf("server settings or acknowledgement missing: %#v", initial)
	}
	entry = initial.Entries[0]
	base := initial.AcceptedVersions["entries"][entryID]
	if err := db.Model(&dao.ExpenseEntry{}).Where("id = ?", entryID).Updates(map[string]any{"note": "remote edit", "updated_at": time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	entry.Note = "local edit"
	entry.UpdatedAt = time.Date(2050, 1, 1, 0, 0, 0, 0, time.UTC)
	conflicted, err := service.Sync(context.Background(), userID, makeRequest(entry, base))
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicted.Conflicts) != 1 || len(conflicted.Entries) != 0 {
		t.Fatalf("stale edit was not held for a choice: %#v", conflicted)
	}
	kept, err := service.Sync(context.Background(), userID, makeRequest(entry, conflicted.Conflicts[0].ServerVersion))
	if err != nil {
		t.Fatal(err)
	}
	if len(kept.Entries) != 1 || kept.Entries[0].Note != "local edit" || kept.Entries[0].UpdatedAt.Year() == 2050 {
		t.Fatalf("local choice did not produce a server-stamped record: %#v", kept)
	}

	// Two devices editing the same base version must produce one accepted write
	// and one conflict, even when the requests overlap.
	base = kept.AcceptedVersions["entries"][entryID]
	entry = kept.Entries[0]
	type outcome struct {
		data *response.SyncData
		err  error
	}
	results := make(chan outcome, 2)
	var wait sync.WaitGroup
	for _, note := range []string{"device A", "device B"} {
		wait.Add(1)
		go func(note string) {
			defer wait.Done()
			row := entry
			row.Note = note
			data, err := service.Sync(context.Background(), userID, makeRequest(row, base))
			results <- outcome{data, err}
		}(note)
	}
	wait.Wait()
	close(results)
	accepted, conflicts := 0, 0
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		accepted += len(result.data.Entries)
		conflicts += len(result.data.Conflicts)
	}
	if accepted != 1 || conflicts != 1 {
		t.Fatalf("concurrent edits: accepted=%d conflicts=%d", accepted, conflicts)
	}

	otherGroup := newID()
	if err := db.Create(&dao.ExpenseGroup{ID: otherGroup, Name: "Other group", InviteCode: "OTHER_TEST"}).Error; err != nil {
		t.Fatal(err)
	}
	foreign := entry
	foreign.ID = newID()
	foreign.GroupID = otherGroup
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	foreign.GroupID = groupID
	foreign.Note = "attempt to move another group's record"
	_, err = service.Sync(context.Background(), userID, makeRequest(foreign, ""))
	if !errors.Is(err, ErrSyncRecordForbidden) {
		t.Fatalf("foreign record was not rejected: %v", err)
	}
}
