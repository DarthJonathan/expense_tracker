package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"expense-tracker/backend/dao"
	"expense-tracker/backend/request"
	"expense-tracker/backend/response"

	"github.com/golang-jwt/jwt/v5"
	uuid "github.com/hashicorp/go-uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	DB              *gorm.DB
	Secret          []byte
	TokenExpiryHour int
}

func NewAuthService(db *gorm.DB, secret string, tokenExpiryHours int) *AuthService {
	expiry := tokenExpiryHours
	if expiry <= 0 {
		expiry = 168
	}
	return &AuthService{DB: db, Secret: []byte(secret), TokenExpiryHour: expiry}
}

type AuthClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

func (s *AuthService) Register(ctx context.Context, req *request.RegisterRequest) (*response.AuthData, error) {
	email := normalizeEmail(req.Email)
	password := strings.TrimSpace(req.Password)
	displayName := strings.TrimSpace(req.DisplayName)

	if email == "" {
		return nil, errors.New("email is required")
	}
	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}

	existing := &dao.ExpenseUser{}
	err := s.DB.WithContext(ctx).Where("email = ? and deleted_at is null", email).First(existing).Error
	if err == nil {
		return nil, errors.New("email is already registered")
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.GenerateUUID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	user := &dao.ExpenseUser{
		ID:           userID,
		Email:        email,
		PasswordHash: string(hashBytes),
		DisplayName:  displayName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.DB.WithContext(ctx).Create(user).Error; err != nil {
		return nil, err
	}

	return s.issueToken(user)
}

func (s *AuthService) Login(ctx context.Context, req *request.LoginRequest) (*response.AuthData, error) {
	email := normalizeEmail(req.Email)
	password := strings.TrimSpace(req.Password)
	if email == "" || password == "" {
		return nil, errors.New("email and password are required")
	}

	user := &dao.ExpenseUser{}
	if err := s.DB.WithContext(ctx).Where("email = ? and deleted_at is null", email).First(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	return s.issueToken(user)
}

func (s *AuthService) AuthenticateWithAPIKey(ctx context.Context, rawKey string) (*response.AuthData, error) {
	user, err := s.AuthenticateAPIKey(ctx, rawKey)
	if err != nil {
		return nil, err
	}

	return s.issueToken(user)
}

func (s *AuthService) CreateAPIKey(ctx context.Context, userID string, req *request.CreateAPIKeyRequest) (*response.APIKeyData, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user is required")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Default key"
	}

	var expiresAt *time.Time
	if strings.TrimSpace(req.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			return nil, errors.New("expiresAt must be RFC3339 format")
		}
		utc := parsed.UTC()
		if utc.Before(time.Now().UTC()) {
			return nil, errors.New("expiresAt must be in the future")
		}
		expiresAt = &utc
	}

	plaintextKey, err := generateAPIKey()
	if err != nil {
		return nil, err
	}
	keyHash := s.hashAPIKey(plaintextKey)
	prefix := apiKeyPrefix(plaintextKey)

	id, err := uuid.GenerateUUID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	key := &dao.ExpenseAPIKey{
		ID:        id,
		UserID:    userID,
		Name:      name,
		KeyPrefix: prefix,
		KeyHash:   keyHash,
		ExpiresAt: expiresAt,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.DB.WithContext(ctx).Create(key).Error; err != nil {
		return nil, err
	}

	out := &response.APIKeyData{
		ID:        key.ID,
		Name:      key.Name,
		Key:       plaintextKey,
		KeyPrefix: key.KeyPrefix,
		CreatedAt: key.CreatedAt.Format(time.RFC3339),
	}
	if key.ExpiresAt != nil {
		out.ExpiresAt = key.ExpiresAt.Format(time.RFC3339)
	}

	return out, nil
}

func (s *AuthService) AuthenticateAPIKey(ctx context.Context, rawKey string) (*dao.ExpenseUser, error) {
	key := strings.TrimSpace(rawKey)
	if key == "" {
		return nil, errors.New("api key is required")
	}

	keyHash := s.hashAPIKey(key)

	stored := &dao.ExpenseAPIKey{}
	err := s.DB.WithContext(ctx).
		Where("key_hash = ? and deleted_at is null and revoked_at is null", keyHash).
		First(stored).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid api key")
		}
		return nil, err
	}

	now := time.Now().UTC()
	if stored.ExpiresAt != nil && stored.ExpiresAt.Before(now) {
		return nil, errors.New("api key expired")
	}

	user := &dao.ExpenseUser{}
	if err := s.DB.WithContext(ctx).
		Where("id = ? and deleted_at is null", stored.UserID).
		First(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid api key")
		}
		return nil, err
	}

	_ = s.DB.WithContext(ctx).Model(stored).Updates(map[string]any{
		"last_used_at": now,
		"updated_at":   now,
	}).Error

	return user, nil
}

func (s *AuthService) ParseToken(tokenString string) (*AuthClaims, error) {
	claims := &AuthClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.Secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.Subject == "" {
		return nil, errors.New("invalid token subject")
	}
	return claims, nil
}

func (s *AuthService) issueToken(user *dao.ExpenseUser) (*response.AuthData, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(s.TokenExpiryHour) * time.Hour)

	claims := &AuthClaims{
		Email: user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.Secret)
	if err != nil {
		return nil, err
	}

	return &response.AuthData{
		Token:     tokenString,
		ExpiresAt: expiresAt.Format(time.RFC3339),
		User: response.AuthUser{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
		},
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func generateAPIKey() (string, error) {
	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(randomBytes)
	return "etk_" + token, nil
}

func apiKeyPrefix(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= 12 {
		return trimmed
	}
	return trimmed[:12]
}

func (s *AuthService) hashAPIKey(rawKey string) string {
	mac := hmac.New(sha256.New, s.Secret)
	_, _ = mac.Write([]byte(strings.TrimSpace(rawKey)))
	return hex.EncodeToString(mac.Sum(nil))
}
