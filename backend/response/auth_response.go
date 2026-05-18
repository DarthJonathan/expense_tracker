package response

type AuthUser struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

type AuthData struct {
	Token     string   `json:"token"`
	ExpiresAt string   `json:"expiresAt"`
	User      AuthUser `json:"user"`
}

type AuthResponse struct {
	BaseResponse
	Data *AuthData `json:"data,omitempty"`
}

type APIKeyData struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key,omitempty"`
	KeyPrefix string `json:"keyPrefix"`
	CreatedAt string `json:"createdAt"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

type APIKeyResponse struct {
	BaseResponse
	Data *APIKeyData `json:"data,omitempty"`
}
