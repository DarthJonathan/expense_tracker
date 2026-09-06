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
	Currency    string          `json:"currency,omitempty"`
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

// CreateStatementIngestionRequest intentionally contains no PDF bytes, data URL,
// extracted page text, or OCR payload. The browser normalizes locally and sends
// only the structured fields needed for collaborative review.
type CreateStatementIngestionRequest struct {
	AccountID                    string                               `json:"accountId"`
	ClientRequestID              string                               `json:"clientRequestId,omitempty"`
	SourceFingerprint            string                               `json:"sourceFingerprint,omitempty"`
	SourceName                   string                               `json:"sourceName"`
	Institution                  string                               `json:"institution,omitempty"`
	StatementCurrency            string                               `json:"statementCurrency"`
	StatementDate                string                               `json:"statementDate,omitempty"`
	PaymentDueDate               string                               `json:"paymentDueDate,omitempty"`
	PeriodStart                  string                               `json:"periodStart,omitempty"`
	PeriodEnd                    string                               `json:"periodEnd,omitempty"`
	PreviousBalance              *int                                 `json:"previousBalance,omitempty"`
	DeclaredNewTransactionsTotal *int                                 `json:"declaredNewTransactionsTotal,omitempty"`
	StatementGrandTotal          *int                                 `json:"statementGrandTotal,omitempty"`
	Warnings                     []string                             `json:"warnings,omitempty"`
	Cardholders                  []StatementCardholderRequest         `json:"cardholders,omitempty"`
	Rows                         []CreateStatementIngestionRowRequest `json:"rows"`
}

type StatementCardholderRequest struct {
	Name             string `json:"name"`
	DeclaredRowCount *int   `json:"declaredRowCount,omitempty"`
	DeclaredSubtotal *int   `json:"declaredSubtotal,omitempty"`
}

type CreateStatementIngestionRowRequest struct {
	SourceRowKey       string   `json:"sourceRowKey"`
	OccurredOn         string   `json:"occurredOn,omitempty"`
	Merchant           string   `json:"merchant"`
	Amount             int      `json:"amount"`
	Currency           string   `json:"currency,omitempty"`
	ForeignAmount      *int     `json:"foreignAmount,omitempty"`
	ForeignCurrency    string   `json:"foreignCurrency,omitempty"`
	StatementKind      string   `json:"statementKind,omitempty"`
	Type               string   `json:"type,omitempty"`
	Cardholder         string   `json:"cardholder,omitempty"`
	StatementReference string   `json:"statementReference,omitempty"`
	AccountID          string   `json:"accountId,omitempty"`
	CategoryID         string   `json:"categoryId,omitempty"`
	Note               string   `json:"note,omitempty"`
	WarningCodes       []string `json:"warningCodes,omitempty"`
}

type UpdateStatementIngestionRowRequest struct {
	OccurredOn      *string   `json:"occurredOn,omitempty"`
	Merchant        *string   `json:"merchant,omitempty"`
	Amount          *int      `json:"amount,omitempty"`
	Currency        *string   `json:"currency,omitempty"`
	ForeignAmount   *int      `json:"foreignAmount,omitempty"`
	ForeignCurrency *string   `json:"foreignCurrency,omitempty"`
	StatementKind   *string   `json:"statementKind,omitempty"`
	Type            *string   `json:"type,omitempty"`
	AccountID       *string   `json:"accountId,omitempty"`
	CategoryID      *string   `json:"categoryId,omitempty"`
	Note            *string   `json:"note,omitempty"`
	ReviewStatus    *string   `json:"reviewStatus,omitempty"`
	MatchExpenseID  *string   `json:"matchExpenseId,omitempty"`
	WarningCodes    *[]string `json:"warningCodes,omitempty"`
}

// CreateCombinedStatementMatchRequest intentionally takes row IDs rather than
// OCR values, keeping the original normalized ingestion rows as the audit trail.
type CreateCombinedStatementMatchRequest struct {
	RowIDs         []string `json:"rowIds"`
	MatchExpenseID string   `json:"matchExpenseId"`
	NewTransaction *CombinedStatementTransactionRequest `json:"newTransaction,omitempty"`
}

type CombinedStatementTransactionRequest struct {
	Merchant string `json:"merchant"`
	OccurredOn string `json:"occurredOn"`
	CategoryID string `json:"categoryId"`
	Note string `json:"note"`
}
