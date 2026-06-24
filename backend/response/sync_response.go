package response

import "expense-tracker/backend/dao"

type SyncSettingsData struct {
	ID            string `json:"id,omitempty"`
	ActiveGroupID string `json:"activeGroupId"`
	DeviceUserID  string `json:"deviceUserId"`
	BaseCurrency  string `json:"baseCurrency"`
	LastSyncedAt  string `json:"lastSyncedAt,omitempty"`
}

type SyncData struct {
	Settings    SyncSettingsData                 `json:"settings"`
	Groups      []dao.ExpenseGroup              `json:"groups"`
	Accounts    []dao.ExpenseAccount            `json:"accounts"`
	Categories  []dao.ExpenseCategory           `json:"categories"`
	Entries     []dao.ExpenseEntry              `json:"entries"`
	Adjustments []dao.ExpenseCategoryAdjustment `json:"adjustments"`
	Merchants   []dao.ExpenseMerchant           `json:"merchants"`
	SyncedAt    string                          `json:"syncedAt"`
}

type SyncResponse struct {
	BaseResponse
	Data *SyncData `json:"data,omitempty"`
}
