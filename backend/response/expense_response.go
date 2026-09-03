package response

import "expense-tracker/backend/dao"

type GroupResponse struct {
	BaseResponse
	Data *dao.ExpenseGroup `json:"data,omitempty"`
}

type AccountListResponse struct {
	BaseResponse
	Data []dao.ExpenseAccount `json:"data,omitempty"`
}

type AccountResponse struct {
	BaseResponse
	Data *dao.ExpenseAccount `json:"data,omitempty"`
}

type CategoryListResponse struct {
	BaseResponse
	Data []dao.ExpenseCategory `json:"data,omitempty"`
}

type CategoryResponse struct {
	BaseResponse
	Data *dao.ExpenseCategory `json:"data,omitempty"`
}

type ExpenseListResponse struct {
	BaseResponse
	Data []dao.ExpenseEntry `json:"data,omitempty"`
}

type ExpenseResponse struct {
	BaseResponse
	Data *dao.ExpenseEntry `json:"data,omitempty"`
}

type AdjustmentListResponse struct {
	BaseResponse
	Data []dao.ExpenseCategoryAdjustment `json:"data,omitempty"`
}

type AdjustmentResponse struct {
	BaseResponse
	Data *dao.ExpenseCategoryAdjustment `json:"data,omitempty"`
}

type StatementIngestionListResponse struct {
	BaseResponse
	Data []dao.ExpenseStatementIngestion `json:"data,omitempty"`
}

type StatementIngestionResponse struct {
	BaseResponse
	Data *StatementIngestionDetail `json:"data,omitempty"`
}

type StatementIngestionDetail struct {
	Ingestion dao.ExpenseStatementIngestion      `json:"ingestion"`
	Rows      []dao.ExpenseStatementIngestionRow `json:"rows"`
}
