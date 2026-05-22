package request

import "encoding/json"

type CreateGroupRequest struct {
	Name      string `json:"name"`
	CreatedBy string `json:"createdBy,omitempty"`
}

type CreateAccountRequest struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	OpeningBalance int    `json:"openingBalance"`
	Color          string `json:"color,omitempty"`
	Icon           string `json:"icon,omitempty"`
}

type CreateCategoryRequest struct {
	Name          string `json:"name"`
	Type          string `json:"type,omitempty"`
	Scope         string `json:"scope,omitempty"`
	Color         string `json:"color,omitempty"`
	Icon          string `json:"icon,omitempty"`
	MonthlyTarget int    `json:"monthlyTarget"`
}

type CreateExpenseRequest struct {
	AccountID  string         `json:"accountId"`
	CategoryID string         `json:"categoryId"`
	Type       string         `json:"type"`
	Amount     int            `json:"amount"`
	Currency   string         `json:"currency,omitempty"`
	OccurredOn string         `json:"occurredOn"`
	Merchant   string         `json:"merchant"`
	Note       string         `json:"note"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type UpdateExpenseRequest struct {
	AccountID  *string `json:"accountId,omitempty"`
	CategoryID *string `json:"categoryId,omitempty"`
	Type       *string `json:"type,omitempty"`
	Amount     *int    `json:"amount,omitempty"`
	Currency   *string `json:"currency,omitempty"`
	OccurredOn *string `json:"occurredOn,omitempty"`
	Merchant   *string `json:"merchant,omitempty"`
	Note       *string `json:"note,omitempty"`
}

type CreateEntryRequest struct {
	AccountID  string `json:"accountId"`
	CategoryID string `json:"categoryId"`
	Type       string `json:"type"`
	Amount     int    `json:"amount"`
	Currency   string `json:"currency,omitempty"`
	OccurredOn string `json:"occurredOn"`
	Merchant   string `json:"merchant"`
	Note       string `json:"note"`
}

type CreateAutomationEntryRequest struct {
	CreatedAt   string          `json:"createdAt"`
	AccountType string          `json:"accountType"`
	Merchant    string          `json:"merchant"`
	Amount      json.RawMessage `json:"amount"`
	Device      string          `json:"device,omitempty"`
}

type ListExpensesRequest struct {
	Query      string
	MonthsBack int
	Type       string
	Limit      int
}

type CreateAdjustmentRequest struct {
	CategoryID string `json:"categoryId"`
	Amount     int    `json:"amount"`
	OccurredOn string `json:"occurredOn"`
	Note       string `json:"note"`
}
