package dao

import "time"

type ExpenseUser struct {
	ID           string     `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Email        string     `gorm:"column:email;type:text;not null;uniqueIndex" json:"email"`
	PasswordHash string     `gorm:"column:password_hash;type:text;not null" json:"-"`
	DisplayName  string     `gorm:"column:display_name;type:text;not null" json:"displayName"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null;default:now()" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;not null;default:now()" json:"updatedAt"`
	DeletedAt    *time.Time `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
}

func (ExpenseUser) TableName() string {
	return QualifiedTable("expense_users")
}

type ExpenseAPIKey struct {
	ID         string     `gorm:"column:id;type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID     string     `gorm:"column:user_id;type:uuid;not null;index" json:"userId"`
	Name       string     `gorm:"column:name;type:text;not null" json:"name"`
	KeyPrefix  string     `gorm:"column:key_prefix;type:text;not null;index" json:"keyPrefix"`
	KeyHash    string     `gorm:"column:key_hash;type:text;not null;uniqueIndex" json:"-"`
	LastUsedAt *time.Time `gorm:"column:last_used_at" json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time `gorm:"column:expires_at" json:"expiresAt,omitempty"`
	RevokedAt  *time.Time `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null;default:now()" json:"createdAt"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;not null;default:now()" json:"updatedAt"`
	DeletedAt  *time.Time `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
}

func (ExpenseAPIKey) TableName() string {
	return QualifiedTable("expense_api_keys")
}
