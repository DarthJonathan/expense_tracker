package request

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
	InviteCode  string `json:"inviteCode"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthenticateAPIKeyRequest struct {
	APIKey string `json:"apiKey"`
}

type CreateAPIKeyRequest struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expiresAt"`
}
