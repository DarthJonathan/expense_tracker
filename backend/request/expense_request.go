package request

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
	Color         string `json:"color,omitempty"`
	Icon          string `json:"icon,omitempty"`
	MonthlyTarget int    `json:"monthlyTarget"`
}

type CreateExpenseRequest struct {
	AccountID  string `json:"accountId"`
	CategoryID string `json:"categoryId"`
	Type       string `json:"type"`
	Amount     int    `json:"amount"`
	Currency   string `json:"currency,omitempty"`
	OccurredOn string `json:"occurredOn"`
	Merchant   string `json:"merchant"`
	Note       string `json:"note"`
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
