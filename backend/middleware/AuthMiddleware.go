package middleware

import (
	"context"
	"net/http"
	"strings"

	"expense-tracker/backend/constants"
	"expense-tracker/backend/service"

	"github.com/gorilla/mux"
)

func AuthMiddleware(authService *service.AuthService) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := strings.TrimSpace(r.Header.Get("Authorization"))
			if header != "" && strings.HasPrefix(strings.ToLower(header), "bearer ") {
				token := strings.TrimSpace(header[7:])
				claims, err := authService.ParseToken(token)
				if err != nil {
					writeUnauthorized(w)
					return
				}

				ctx := context.WithValue(r.Context(), constants.AuthUserIDCtx, claims.Subject)
				ctx = context.WithValue(ctx, constants.AuthEmailCtx, claims.Email)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			apiKey := strings.TrimSpace(r.Header.Get("X-API-Key"))
			if apiKey != "" {
				user, err := authService.AuthenticateAPIKey(r.Context(), apiKey)
				if err != nil {
					writeUnauthorized(w)
					return
				}

				ctx := context.WithValue(r.Context(), constants.AuthUserIDCtx, user.ID)
				ctx = context.WithValue(ctx, constants.AuthEmailCtx, user.Email)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			writeUnauthorized(w)
		})
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"success":false,"error":"unauthorized"}`))
}
