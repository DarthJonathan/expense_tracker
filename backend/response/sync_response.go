package response

import "expense-tracker/backend/dao"

type SyncData struct {
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
