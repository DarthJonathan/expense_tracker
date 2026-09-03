package dao

import "time"

type ExpenseGroup struct {
	ID         string     `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name       string     `gorm:"column:name;type:text;not null" json:"name"`
	InviteCode string     `gorm:"column:invite_code;type:text;not null;uniqueIndex" json:"inviteCode"`
	CreatedBy  *string    `gorm:"column:created_by;type:uuid" json:"createdBy,omitempty"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null;default:now()" json:"createdAt"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;not null;default:now()" json:"updatedAt"`
	DeletedAt  *time.Time `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
}

func (ExpenseGroup) TableName() string { return QualifiedTable("expense_groups") }

type ExpenseAccount struct {
	ID             string     `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GroupID        string     `gorm:"column:group_id;type:uuid;not null;index" json:"groupId"`
	Name           string     `gorm:"column:name;type:text;not null" json:"name"`
	Type           string     `gorm:"column:type;type:text;not null;check:type in ('cash','bank','card','wallet')" json:"type"`
	OpeningBalance int        `gorm:"column:opening_balance;not null;default:0" json:"openingBalance"`
	Color          string     `gorm:"column:color;type:text;not null;default:'#4b5745'" json:"color"`
	Icon           string     `gorm:"column:icon;type:text;not null;default:'🏦'" json:"icon"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null;default:now()" json:"createdAt"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null;default:now()" json:"updatedAt"`
	DeletedAt      *time.Time `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
}

func (ExpenseAccount) TableName() string { return QualifiedTable("expense_accounts") }

type ExpenseCategory struct {
	ID            string     `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GroupID       string     `gorm:"column:group_id;type:uuid;not null;index" json:"groupId"`
	Name          string     `gorm:"column:name;type:text;not null" json:"name"`
	Type          string     `gorm:"column:type;type:text;not null;default:'expense';check:type in ('expense','income')" json:"type"`
	Scope         string     `gorm:"column:scope;type:text;not null;default:'household';check:scope in ('household','user')" json:"scope"`
	OwnerUserID   *string    `gorm:"column:owner_user_id;type:uuid;index" json:"ownerUserId,omitempty"`
	Color         string     `gorm:"column:color;type:text;not null;default:'#e7d24e'" json:"color"`
	Icon          string     `gorm:"column:icon;type:text;not null;default:'🏷️'" json:"icon"`
	MonthlyTarget int        `gorm:"column:monthly_target;not null;default:0" json:"monthlyTarget"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null;default:now()" json:"createdAt"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;not null;default:now()" json:"updatedAt"`
	DeletedAt     *time.Time `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
}

func (ExpenseCategory) TableName() string { return QualifiedTable("expense_categories") }

type ExpenseEntry struct {
	ID           string         `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GroupID      string         `gorm:"column:group_id;type:uuid;not null;index" json:"groupId"`
	AccountID    string         `gorm:"column:account_id;type:uuid;not null;index" json:"accountId"`
	CategoryID   string         `gorm:"column:category_id;type:uuid;not null;index" json:"categoryId"`
	Type         string         `gorm:"column:type;type:text;not null;check:type in ('expense','income')" json:"type"`
	Amount       int            `gorm:"column:amount;not null;check:amount >= 0" json:"amount"`
	Currency     string         `gorm:"column:currency;type:text;not null;default:'SGD'" json:"currency"`
	BaseAmount   int            `gorm:"column:base_amount;not null;default:0" json:"baseAmount"`
	BaseCurrency string         `gorm:"column:base_currency;type:text;not null;default:'SGD'" json:"baseCurrency"`
	FxRate       float64        `gorm:"column:fx_rate;type:numeric(20,10);not null;default:1" json:"fxRate"`
	FxRateDate   string         `gorm:"column:fx_rate_date;type:date;not null" json:"fxRateDate"`
	OccurredOn   string         `gorm:"column:occurred_on;type:date;not null" json:"occurredOn"`
	Merchant     string         `gorm:"column:merchant;type:text;not null" json:"merchant"`
	Note         string         `gorm:"column:note;type:text;not null;default:''" json:"note"`
	Metadata     map[string]any `gorm:"column:metadata;type:jsonb;serializer:json;not null;default:'{}'" json:"metadata"`
	CreatedBy    *string        `gorm:"column:created_by;type:uuid" json:"createdBy,omitempty"`
	CreatedAt    time.Time      `gorm:"column:created_at;not null;default:now()" json:"createdAt"`
	UpdatedAt    time.Time      `gorm:"column:updated_at;not null;default:now()" json:"updatedAt"`
	DeletedAt    *time.Time     `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
}

func (ExpenseEntry) TableName() string { return QualifiedTable("expense_entries") }

type ExpenseCategoryAdjustment struct {
	ID         string     `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GroupID    string     `gorm:"column:group_id;type:uuid;not null;index" json:"groupId"`
	CategoryID string     `gorm:"column:category_id;type:uuid;not null;index" json:"categoryId"`
	Amount     int        `gorm:"column:amount;not null" json:"amount"`
	OccurredOn string     `gorm:"column:occurred_on;type:date;not null" json:"occurredOn"`
	Note       string     `gorm:"column:note;type:text;not null;default:''" json:"note"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null;default:now()" json:"createdAt"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;not null;default:now()" json:"updatedAt"`
	DeletedAt  *time.Time `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
}

func (ExpenseCategoryAdjustment) TableName() string {
	return QualifiedTable("expense_category_adjustments")
}

type ExpenseMerchant struct {
	ID             string     `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GroupID        string     `gorm:"column:group_id;type:uuid;not null;index" json:"groupId"`
	Name           string     `gorm:"column:name;type:text;not null" json:"name"`
	NormalizedName string     `gorm:"column:normalized_name;type:text;not null;index" json:"normalizedName"`
	UsageCount     int        `gorm:"column:usage_count;not null;default:0" json:"usageCount"`
	LastUsedAt     *time.Time `gorm:"column:last_used_at" json:"lastUsedAt,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null;default:now()" json:"createdAt"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null;default:now()" json:"updatedAt"`
	DeletedAt      *time.Time `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
}

func (ExpenseMerchant) TableName() string {
	return QualifiedTable("expense_merchants")
}

type ExpenseMerchantCategoryMap struct {
	ID                 string     `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GroupID            string     `gorm:"column:group_id;type:uuid;not null;index" json:"groupId"`
	NormalizedMerchant string     `gorm:"column:normalized_merchant;type:text;not null;index" json:"normalizedMerchant"`
	EntryType          string     `gorm:"column:entry_type;type:text;not null;default:'expense';check:entry_type in ('expense','income')" json:"entryType"`
	CategoryID         string     `gorm:"column:category_id;type:uuid;not null;index" json:"categoryId"`
	Confidence         float64    `gorm:"column:confidence;type:numeric(4,3);not null;default:1.000" json:"confidence"`
	Source             string     `gorm:"column:source;type:text;not null;default:'learned'" json:"source"`
	HitCount           int        `gorm:"column:hit_count;not null;default:0" json:"hitCount"`
	LastSeenAt         *time.Time `gorm:"column:last_seen_at" json:"lastSeenAt,omitempty"`
	CreatedAt          time.Time  `gorm:"column:created_at;not null;default:now()" json:"createdAt"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;not null;default:now()" json:"updatedAt"`
	DeletedAt          *time.Time `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
}

func (ExpenseMerchantCategoryMap) TableName() string {
	return QualifiedTable("expense_merchant_category_maps")
}

type ExpenseCategoryRule struct {
	ID         string     `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GroupID    string     `gorm:"column:group_id;type:uuid;not null;index" json:"groupId"`
	Priority   int        `gorm:"column:priority;not null;default:100" json:"priority"`
	Enabled    bool       `gorm:"column:enabled;not null;default:true" json:"enabled"`
	EntryType  string     `gorm:"column:entry_type;type:text;not null;default:'any';check:entry_type in ('expense','income','any')" json:"entryType"`
	MatchField string     `gorm:"column:match_field;type:text;not null;default:'merchant';check:match_field in ('merchant','note','account_type')" json:"matchField"`
	MatchKind  string     `gorm:"column:match_kind;type:text;not null;default:'contains';check:match_kind in ('contains','prefix','equals','regex')" json:"matchKind"`
	Pattern    string     `gorm:"column:pattern;type:text;not null" json:"pattern"`
	CategoryID string     `gorm:"column:category_id;type:uuid;not null;index" json:"categoryId"`
	Confidence float64    `gorm:"column:confidence;type:numeric(4,3);not null;default:0.900" json:"confidence"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null;default:now()" json:"createdAt"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;not null;default:now()" json:"updatedAt"`
	DeletedAt  *time.Time `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
}

func (ExpenseCategoryRule) TableName() string {
	return QualifiedTable("expense_category_rules")
}

// ExpenseStatementIngestion is deliberately limited to normalized statement data.
// The source PDF and page text never have a column in this model: those stay in the
// browser's IndexedDB while a client-side job is running.
type ExpenseStatementIngestion struct {
	ID                           string           `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GroupID                      string           `gorm:"column:group_id;type:uuid;not null;index" json:"groupId"`
	AccountID                    string           `gorm:"column:account_id;type:uuid;not null;index" json:"accountId"`
	ClientRequestID              string           `gorm:"column:client_request_id;type:text;not null;default:''" json:"clientRequestId,omitempty"`
	SourceFingerprint            string           `gorm:"column:source_fingerprint;type:text;not null;default:''" json:"sourceFingerprint,omitempty"`
	Status                       string           `gorm:"column:status;type:text;not null;index" json:"status"`
	SourceName                   string           `gorm:"column:source_name;type:text;not null" json:"sourceName"`
	Institution                  string           `gorm:"column:institution;type:text;not null;default:''" json:"institution"`
	StatementCurrency            string           `gorm:"column:statement_currency;type:text;not null;default:'SGD'" json:"statementCurrency"`
	StatementDate                *string          `gorm:"column:statement_date;type:date" json:"statementDate,omitempty"`
	PaymentDueDate               *string          `gorm:"column:payment_due_date;type:date" json:"paymentDueDate,omitempty"`
	PeriodStart                  *string          `gorm:"column:period_start;type:date" json:"periodStart,omitempty"`
	PeriodEnd                    *string          `gorm:"column:period_end;type:date" json:"periodEnd,omitempty"`
	PreviousBalance              *int             `gorm:"column:previous_balance" json:"previousBalance,omitempty"`
	DeclaredNewTransactionsTotal *int             `gorm:"column:declared_new_transactions_total" json:"declaredNewTransactionsTotal,omitempty"`
	StatementGrandTotal          *int             `gorm:"column:statement_grand_total" json:"statementGrandTotal,omitempty"`
	ParsedRowCount               int              `gorm:"column:parsed_row_count;not null;default:0" json:"parsedRowCount"`
	CardholderControls           []map[string]any `gorm:"column:cardholder_controls;type:jsonb;serializer:json;not null;default:'[]'" json:"cardholderControls"`
	Validation                   map[string]any   `gorm:"column:validation;type:jsonb;serializer:json;not null;default:'{}'" json:"validation"`
	Warnings                     []string         `gorm:"column:warnings;type:jsonb;serializer:json;not null;default:'[]'" json:"warnings"`
	CreatedBy                    *string          `gorm:"column:created_by;type:uuid" json:"createdBy,omitempty"`
	ConfirmedAt                  *time.Time       `gorm:"column:confirmed_at" json:"confirmedAt,omitempty"`
	CreatedAt                    time.Time        `gorm:"column:created_at;not null;default:now()" json:"createdAt"`
	UpdatedAt                    time.Time        `gorm:"column:updated_at;not null;default:now()" json:"updatedAt"`
	DeletedAt                    *time.Time       `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
	Group                        *ExpenseGroup    `gorm:"foreignKey:GroupID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Account                      *ExpenseAccount  `gorm:"foreignKey:AccountID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
	Creator                      *ExpenseUser     `gorm:"foreignKey:CreatedBy;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"-"`
}

func (ExpenseStatementIngestion) TableName() string {
	return QualifiedTable("expense_statement_ingestions")
}

// ExpenseStatementIngestionRow has its own primary key. A match cannot be the
// primary key because unmatched rows have no match. Separate partial uniqueness
// prevents two active rows in one ingestion from claiming the same expense.
type ExpenseStatementIngestionRow struct {
	ID                 string                     `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	IngestionID        string                     `gorm:"column:ingestion_id;type:uuid;not null;index" json:"ingestionId"`
	GroupID            string                     `gorm:"column:group_id;type:uuid;not null;index" json:"groupId"`
	SourceRowKey       string                     `gorm:"column:source_row_key;type:text;not null" json:"sourceRowKey"`
	OccurredOn         *string                    `gorm:"column:occurred_on;type:date" json:"occurredOn,omitempty"`
	Merchant           string                     `gorm:"column:merchant;type:text;not null;default:''" json:"merchant"`
	Amount             int                        `gorm:"column:amount;not null;default:0;check:amount >= 0" json:"amount"`
	Currency           string                     `gorm:"column:currency;type:text;not null;default:'SGD'" json:"currency"`
	ForeignAmount      *int                       `gorm:"column:foreign_amount;check:foreign_amount is null or foreign_amount >= 0" json:"foreignAmount,omitempty"`
	ForeignCurrency    string                     `gorm:"column:foreign_currency;type:text;not null;default:''" json:"foreignCurrency"`
	StatementKind      string                     `gorm:"column:statement_kind;type:text;not null;default:'transaction'" json:"statementKind"`
	Type               string                     `gorm:"column:type;type:text;not null;default:'expense'" json:"type"`
	Cardholder         string                     `gorm:"column:cardholder;type:text;not null;default:''" json:"cardholder"`
	StatementReference string                     `gorm:"column:statement_reference;type:text;not null;default:''" json:"statementReference"`
	AccountID          *string                    `gorm:"column:account_id;type:uuid" json:"accountId,omitempty"`
	CategoryID         *string                    `gorm:"column:category_id;type:uuid" json:"categoryId,omitempty"`
	Note               string                     `gorm:"column:note;type:text;not null;default:''" json:"note"`
	ReviewStatus       string                     `gorm:"column:review_status;type:text;not null;default:'unreviewed';index" json:"reviewStatus"`
	SuggestedExpenseID *string                    `gorm:"column:suggested_expense_id;type:uuid;index" json:"suggestedExpenseId,omitempty"`
	MatchConfidence    *float64                   `gorm:"column:match_confidence;type:numeric(4,3)" json:"matchConfidence,omitempty"`
	MatchExpenseID     *string                    `gorm:"column:match_expense_id;type:uuid;index" json:"matchExpenseId,omitempty"`
	ConfirmedExpenseID *string                    `gorm:"column:confirmed_expense_id;type:uuid;index" json:"confirmedExpenseId,omitempty"`
	WarningCodes       []string                   `gorm:"column:warning_codes;type:jsonb;serializer:json;not null;default:'[]'" json:"warningCodes"`
	ReviewedBy         *string                    `gorm:"column:reviewed_by;type:uuid" json:"reviewedBy,omitempty"`
	ReviewedAt         *time.Time                 `gorm:"column:reviewed_at" json:"reviewedAt,omitempty"`
	CreatedAt          time.Time                  `gorm:"column:created_at;not null;default:now()" json:"createdAt"`
	UpdatedAt          time.Time                  `gorm:"column:updated_at;not null;default:now()" json:"updatedAt"`
	DeletedAt          *time.Time                 `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
	Ingestion          *ExpenseStatementIngestion `gorm:"foreignKey:IngestionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Group              *ExpenseGroup              `gorm:"foreignKey:GroupID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Account            *ExpenseAccount            `gorm:"foreignKey:AccountID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"-"`
	Category           *ExpenseCategory           `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"-"`
	SuggestedExpense   *ExpenseEntry              `gorm:"foreignKey:SuggestedExpenseID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"-"`
	MatchedExpense     *ExpenseEntry              `gorm:"foreignKey:MatchExpenseID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"-"`
	ConfirmedExpense   *ExpenseEntry              `gorm:"foreignKey:ConfirmedExpenseID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"-"`
	Reviewer           *ExpenseUser               `gorm:"foreignKey:ReviewedBy;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"-"`
}

func (ExpenseStatementIngestionRow) TableName() string {
	return QualifiedTable("expense_statement_ingestion_rows")
}
