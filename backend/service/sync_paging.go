package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"expense-tracker/backend/dao"
	"expense-tracker/backend/request"
	"expense-tracker/backend/response"

	uuid "github.com/hashicorp/go-uuid"
	"gorm.io/gorm"
)

var ErrInvalidSyncOptions = errors.New("invalid sync options")

func validateSyncOptions(options *request.SyncOptions) error {
	if options == nil {
		return nil // Existing clients retain the original protocol.
	}
	if options.Mode != "push" && options.Mode != "pull" {
		return fmt.Errorf("%w: mode must be push or pull", ErrInvalidSyncOptions)
	}
	if options.Version != 0 && options.Version != 3 {
		return fmt.Errorf("%w: unsupported protocol version", ErrInvalidSyncOptions)
	}
	if options.Mode == "push" {
		return nil
	}
	switch options.Collection {
	case "groups", "accounts", "categories", "entries", "adjustments", "merchants":
	default:
		return fmt.Errorf("%w: unknown collection", ErrInvalidSyncOptions)
	}
	if options.Cursor != "" {
		if _, err := uuid.ParseUUID(options.Cursor); err != nil {
			return fmt.Errorf("%w: cursor must be a UUID", ErrInvalidSyncOptions)
		}
	}
	if options.Limit < 0 || options.Limit > 200 {
		return fmt.Errorf("%w: limit must be between 1 and 200", ErrInvalidSyncOptions)
	}
	if len(options.Known) > 200 {
		return fmt.Errorf("%w: at most 200 known versions are allowed", ErrInvalidSyncOptions)
	}
	return nil
}

func syncPageLimit(options *request.SyncOptions) int {
	if options.Limit == 0 {
		return 100
	}
	return options.Limit
}

func syncReadQuery(query string, args []any, pages []*request.SyncOptions) (string, []any) {
	if len(pages) == 0 || pages[0] == nil {
		return query, args
	}
	options := pages[0]
	if order := strings.Index(strings.ToLower(query), "order by"); order >= 0 {
		query = query[:order]
	}
	if options.Cursor != "" {
		query += " and id > ?::uuid"
		args = append(args, options.Cursor)
	}
	if options.RecordID != "" {
		query += " and id = ?::uuid"
		args = append(args, options.RecordID)
	}
	// The SELECT projects id as text. An explicit UUID expression uses the
	// underlying UUID column for ordering and keeps the (group_id, id) index usable.
	query += " order by id::uuid asc limit ?"
	args = append(args, syncPageLimit(options)+1)
	if options.Lock {
		query += " for update"
	}
	return query, args
}

func syncQuery(tx *gorm.DB, pages []*request.SyncOptions, query string, args ...any) *gorm.DB {
	query, args = syncReadQuery(query, args, pages)
	return tx.Raw(query, args...)
}

func trimSyncPage[T any](rows []T, options *request.SyncOptions, id func(T) string) ([]T, *response.SyncPage) {
	limit := syncPageLimit(options)
	page := &response.SyncPage{Collection: options.Collection, HasMore: len(rows) > limit, Versions: map[string]string{}}
	if page.HasMore {
		rows = rows[:limit]
		page.NextCursor = id(rows[len(rows)-1])
	}
	changed := make([]T, 0, len(rows))
	for _, row := range rows {
		// Compare server-issued content versions, rather than trusting device clocks.
		version := syncRecordVersion(row)
		page.Versions[id(row)] = version
		if options.Known[id(row)] != version {
			changed = append(changed, row)
		}
	}
	return changed, page
}

func syncRecordVersion(row any) string {
	encoded, _ := json.Marshal(row)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func pullSyncPage(tx *gorm.DB, groupID, userID string, options *request.SyncOptions, result *response.SyncData) error {
	var err error
	switch options.Collection {
	case "groups":
		result.Groups, err = pullGroups(tx, groupID, options)
		result.Groups, result.Page = trimSyncPage(result.Groups, options, func(row dao.ExpenseGroup) string { return row.ID })
	case "accounts":
		result.Accounts, err = pullAccounts(tx, groupID, options)
		result.Accounts, result.Page = trimSyncPage(result.Accounts, options, func(row dao.ExpenseAccount) string { return row.ID })
	case "categories":
		result.Categories, err = pullCategories(tx, groupID, userID, options)
		result.Categories, result.Page = trimSyncPage(result.Categories, options, func(row dao.ExpenseCategory) string { return row.ID })
	case "entries":
		result.Entries, err = pullEntries(tx, groupID, options)
		result.Entries, result.Page = trimSyncPage(result.Entries, options, func(row dao.ExpenseEntry) string { return row.ID })
	case "adjustments":
		result.Adjustments, err = pullAdjustments(tx, groupID, options)
		result.Adjustments, result.Page = trimSyncPage(result.Adjustments, options, func(row dao.ExpenseCategoryAdjustment) string { return row.ID })
	case "merchants":
		result.Merchants, err = pullMerchants(tx, groupID, options)
		result.Merchants, result.Page = trimSyncPage(result.Merchants, options, func(row dao.ExpenseMerchant) string { return row.ID })
	}
	if err != nil {
		return fmt.Errorf("pull %s page: %w", options.Collection, err)
	}
	return nil
}
