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
	Conflicts        []SyncConflict                  `json:"conflicts,omitempty"`
	AcceptedVersions map[string]map[string]string    `json:"acceptedVersions,omitempty"`
	ProtocolVersion  int                             `json:"protocolVersion,omitempty"`
	Page             *SyncPage                       `json:"page,omitempty"`
	Settings         SyncSettingsData                `json:"settings"`
	Groups           []dao.ExpenseGroup              `json:"groups"`
	Accounts         []dao.ExpenseAccount            `json:"accounts"`
	Categories       []dao.ExpenseCategory           `json:"categories"`
	Entries          []dao.ExpenseEntry              `json:"entries"`
	Adjustments      []dao.ExpenseCategoryAdjustment `json:"adjustments"`
	Merchants        []dao.ExpenseMerchant           `json:"merchants"`
	SyncedAt         string                          `json:"syncedAt"`
}

type SyncConflict struct {
	Collection    string `json:"collection"`
	ID            string `json:"id"`
	ServerVersion string `json:"serverVersion"`
	Server        any    `json:"server"`
}

type SyncPage struct {
	Collection string            `json:"collection"`
	NextCursor string            `json:"nextCursor,omitempty"`
	HasMore    bool              `json:"hasMore"`
	Versions   map[string]string `json:"versions"`
}

type SyncResponse struct {
	BaseResponse
	Data               *SyncData            `json:"data,omitempty"`
	InaccessibleRecord *SyncRecordReference `json:"inaccessibleRecord,omitempty"`
}

// Contains only the collection and UUID submitted by this client.
type SyncRecordReference struct {
	Collection string `json:"collection"`
	ID         string `json:"id"`
}
