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
	ID         string     `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	GroupID    string     `gorm:"column:group_id;type:uuid;not null;index" json:"groupId"`
	AccountID  string     `gorm:"column:account_id;type:uuid;not null;index" json:"accountId"`
	CategoryID string     `gorm:"column:category_id;type:uuid;not null;index" json:"categoryId"`
	Type       string     `gorm:"column:type;type:text;not null;check:type in ('expense','income')" json:"type"`
	Amount     int        `gorm:"column:amount;not null;check:amount >= 0" json:"amount"`
	Currency   string     `gorm:"column:currency;type:text;not null;default:'SGD'" json:"currency"`
	OccurredOn string     `gorm:"column:occurred_on;type:date;not null" json:"occurredOn"`
	Merchant   string     `gorm:"column:merchant;type:text;not null" json:"merchant"`
	Note       string     `gorm:"column:note;type:text;not null;default:''" json:"note"`
	CreatedBy  *string    `gorm:"column:created_by;type:uuid" json:"createdBy,omitempty"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null;default:now()" json:"createdAt"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;not null;default:now()" json:"updatedAt"`
	DeletedAt  *time.Time `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
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
