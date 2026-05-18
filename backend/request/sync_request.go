package request

import "expense-tracker/backend/dao"

type SyncSettingsRequest struct {
	ID            string  `json:"id,omitempty"`
	ActiveGroupID string  `json:"activeGroupId"`
	DeviceUserID  string  `json:"deviceUserId"`
	LastSyncedAt  *string `json:"lastSyncedAt,omitempty"`
}

type SyncRequest struct {
	Settings    SyncSettingsRequest             `json:"settings"`
	Groups      []dao.ExpenseGroup              `json:"groups"`
	Accounts    []dao.ExpenseAccount            `json:"accounts"`
	Categories  []dao.ExpenseCategory           `json:"categories"`
	Entries     []dao.ExpenseEntry              `json:"entries"`
	Adjustments []dao.ExpenseCategoryAdjustment `json:"adjustments"`
	Merchants   []dao.ExpenseMerchant           `json:"merchants"`
}
