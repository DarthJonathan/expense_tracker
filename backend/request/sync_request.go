package request

import "expense-tracker/backend/dao"

type SyncSettingsRequest struct {
	ID            string  `json:"id,omitempty"`
	ActiveGroupID string  `json:"activeGroupId"`
	DeviceUserID  string  `json:"deviceUserId"`
	BaseCurrency  string  `json:"baseCurrency,omitempty"`
	LastSyncedAt  *string `json:"lastSyncedAt,omitempty"`
}

type SyncRequest struct {
	Sync        *SyncOptions                    `json:"sync,omitempty"`
	Settings    SyncSettingsRequest             `json:"settings"`
	Groups      []dao.ExpenseGroup              `json:"groups"`
	Accounts    []dao.ExpenseAccount            `json:"accounts"`
	Categories  []dao.ExpenseCategory           `json:"categories"`
	Entries     []dao.ExpenseEntry              `json:"entries"`
	Adjustments []dao.ExpenseCategoryAdjustment `json:"adjustments"`
	Merchants   []dao.ExpenseMerchant           `json:"merchants"`
}

type SyncOptions struct {
	Version      int                          `json:"version,omitempty"`
	BaseVersions map[string]map[string]string `json:"baseVersions,omitempty"`
	Mode         string                       `json:"mode"`
	Collection   string                       `json:"collection,omitempty"`
	Cursor       string                       `json:"cursor,omitempty"`
	Limit        int                          `json:"limit,omitempty"`
	Known        map[string]string            `json:"known,omitempty"`
	RecordID     string                       `json:"-"`
	Lock         bool                         `json:"-"`
}
