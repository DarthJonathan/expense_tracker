package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"expense-tracker/backend/dao"
	"expense-tracker/backend/request"
	"expense-tracker/backend/response"

	"gorm.io/gorm"
)

var ErrSyncUpgradeRequired = errors.New("update the app to resolve sync conflicts")
var ErrSyncRecordForbidden = errors.New("sync record is not accessible")

// These are user-editable fields. Identity, ownership, timestamps and FX values
// are canonical server fields and must not create conflicts on their own.
var syncEditableFields = map[string][]string{
	"groups":      {"name", "deletedAt"},
	"accounts":    {"name", "type", "openingBalance", "fxMarkupPercent", "color", "icon", "deletedAt"},
	"categories":  {"name", "type", "scope", "color", "icon", "monthlyTarget", "deletedAt"},
	"entries":     {"accountId", "categoryId", "type", "amount", "currency", "occurredOn", "merchant", "note", "metadata", "deletedAt"},
	"adjustments": {"categoryId", "amount", "occurredOn", "note", "deletedAt"},
}

func sameSyncContent(collection string, incoming, current any) bool {
	project := func(record any) map[string]any {
		encoded, _ := json.Marshal(record)
		var values map[string]any
		_ = json.Unmarshal(encoded, &values)
		selected := map[string]any{}
		for _, key := range syncEditableFields[collection] {
			value := values[key]
			if key == "deletedAt" {
				value = value != nil && value != ""
			}
			if key == "metadata" {
				metadata, _ := value.(map[string]any)
				if metadata == nil {
					metadata = map[string]any{}
				}
				delete(metadata, "fxMarkupPercent")
				value = metadata
			}
			selected[key] = value
		}
		return selected
	}
	return reflect.DeepEqual(project(incoming), project(current))
}

func syncWriteConflicts(collection string, incoming, current any, baseVersion string) bool {
	if current == nil {
		return baseVersion != "" && baseVersion != "missing"
	}
	return !sameSyncContent(collection, incoming, current) && baseVersion != syncRecordVersion(current)
}

func readSyncRecord(tx *gorm.DB, collection, id, groupID, userID string) (any, error) {
	options := &request.SyncOptions{Mode: "pull", Collection: collection, RecordID: id, Limit: 1, Lock: true}
	result := &response.SyncData{}
	if err := pullSyncPage(tx, groupID, userID, options, result); err != nil {
		return nil, err
	}
	var rows any
	switch collection {
	case "groups":
		rows = result.Groups
	case "accounts":
		rows = result.Accounts
	case "categories":
		rows = result.Categories
	case "entries":
		rows = result.Entries
	case "adjustments":
		rows = result.Adjustments
	}
	value := reflect.ValueOf(rows)
	if value.IsValid() && value.Len() > 0 {
		return value.Index(0).Interface(), nil
	}
	// A UUID already belonging to another group or a private category cannot be
	// treated as a new record and moved through ON CONFLICT.
	tables := map[string]string{"groups": "expense_groups", "accounts": "expense_accounts", "categories": "expense_categories", "entries": "expense_entries", "adjustments": "expense_category_adjustments"}
	var count int64
	if err := tx.Table(dao.QualifiedTable(tables[collection])).Where("id = ?::uuid", id).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSyncRecordForbidden
	}
	return nil, nil
}

func stampSyncRecord[T any](record T, current any, now time.Time) T {
	value := reflect.ValueOf(&record).Elem()
	value.FieldByName("UpdatedAt").Set(reflect.ValueOf(now))
	value.FieldByName("CreatedAt").Set(reflect.ValueOf(now))
	if current != nil {
		stored := reflect.ValueOf(current)
		for _, name := range []string{"CreatedAt", "CreatedBy", "InviteCode"} {
			field, source := value.FieldByName(name), stored.FieldByName(name)
			if field.IsValid() && source.IsValid() {
				field.Set(source)
			}
		}
	}
	return record
}

func prepareSyncCollection[T any](tx *gorm.DB, collection string, records []T, id func(T) string, groupID, userID string, options *request.SyncOptions, result *response.SyncData, touched map[string][]string, now time.Time) ([]T, error) {
	sort.Slice(records, func(i, j int) bool { return id(records[i]) < id(records[j]) })
	accepted := make([]T, 0, len(records))
	for _, record := range records {
		recordID := id(record)
		// Serialize competing creations too; FOR UPDATE only locks existing rows.
		if err := tx.Exec("select pg_advisory_xact_lock(hashtextextended(?, 0))", collection+":"+recordID).Error; err != nil {
			return nil, err
		}
		current, err := readSyncRecord(tx, collection, recordID, groupID, userID)
		if err != nil {
			return nil, err
		}
		baseVersion := ""
		if options != nil {
			baseVersion = options.BaseVersions[collection][recordID]
		}
		if syncWriteConflicts(collection, record, current, baseVersion) {
			if options == nil || options.Version != 3 {
				return nil, ErrSyncUpgradeRequired
			}
			version := "missing"
			if current != nil {
				version = syncRecordVersion(current)
			}
			result.Conflicts = append(result.Conflicts, response.SyncConflict{Collection: collection, ID: recordID, Server: current, ServerVersion: version})
			continue
		}
		touched[collection] = append(touched[collection], recordID)
		if current != nil && sameSyncContent(collection, record, current) {
			continue
		}
		stamped := stampSyncRecord(record, current, now)
		if current == nil {
			creator := reflect.ValueOf(&stamped).Elem().FieldByName("CreatedBy")
			if creator.IsValid() {
				creator.Set(reflect.ValueOf(&userID))
			}
		}
		accepted = append(accepted, stamped)
	}
	return accepted, nil
}

func prepareSyncPush(tx *gorm.DB, req *request.SyncRequest, groupID, sourceGroupID, userID string, result *response.SyncData, now time.Time) (map[string][]string, error) {
	touched := map[string][]string{}
	var err error
	groups := []dao.ExpenseGroup{}
	for _, group := range req.Groups {
		if group.ID == sourceGroupID {
			group.ID = groupID
			groups = append(groups, group)
		}
	}
	req.Groups, err = prepareSyncCollection(tx, "groups", groups, func(r dao.ExpenseGroup) string { return r.ID }, groupID, userID, req.Sync, result, touched, now)
	if err != nil {
		return nil, err
	}
	req.Accounts, err = prepareSyncCollection(tx, "accounts", filterAccountsByGroup(req.Accounts, sourceGroupID, groupID), func(r dao.ExpenseAccount) string { return r.ID }, groupID, userID, req.Sync, result, touched, now)
	if err != nil {
		return nil, err
	}
	req.Categories, err = prepareSyncCollection(tx, "categories", filterCategoriesByGroup(req.Categories, sourceGroupID, groupID, userID), func(r dao.ExpenseCategory) string { return r.ID }, groupID, userID, req.Sync, result, touched, now)
	if err != nil {
		return nil, err
	}
	req.Entries, err = prepareSyncCollection(tx, "entries", filterEntriesByGroup(req.Entries, sourceGroupID, groupID), func(r dao.ExpenseEntry) string { return r.ID }, groupID, userID, req.Sync, result, touched, now)
	if err != nil {
		return nil, err
	}
	req.Adjustments, err = prepareSyncCollection(tx, "adjustments", filterAdjustmentsByGroup(req.Adjustments, sourceGroupID, groupID), func(r dao.ExpenseCategoryAdjustment) string { return r.ID }, groupID, userID, req.Sync, result, touched, now)
	if err != nil {
		return nil, err
	}
	// The merchant catalogue is derived from accepted entries on the server.
	req.Merchants = nil
	return touched, nil
}

func addAcceptedSyncRecords(tx *gorm.DB, result *response.SyncData, touched map[string][]string, groupID, userID string) error {
	result.AcceptedVersions = map[string]map[string]string{}
	for _, collection := range []string{"groups", "accounts", "categories", "entries", "adjustments"} {
		for _, id := range touched[collection] {
			record, err := readSyncRecord(tx, collection, id, groupID, userID)
			if err != nil {
				return err
			}
			if record == nil {
				return fmt.Errorf("accepted %s record is missing", collection)
			}
			if result.AcceptedVersions[collection] == nil {
				result.AcceptedVersions[collection] = map[string]string{}
			}
			result.AcceptedVersions[collection][id] = syncRecordVersion(record)
			switch collection {
			case "groups":
				result.Groups = append(result.Groups, record.(dao.ExpenseGroup))
			case "accounts":
				result.Accounts = append(result.Accounts, record.(dao.ExpenseAccount))
			case "categories":
				result.Categories = append(result.Categories, record.(dao.ExpenseCategory))
			case "entries":
				result.Entries = append(result.Entries, record.(dao.ExpenseEntry))
			case "adjustments":
				result.Adjustments = append(result.Adjustments, record.(dao.ExpenseCategoryAdjustment))
			}
		}
	}
	return nil
}
