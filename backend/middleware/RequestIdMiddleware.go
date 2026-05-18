package middleware

import (
	"context"
	"net/http"

	"expense-tracker/backend/constants"

	"github.com/hashicorp/go-uuid"
)

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			generated, err := uuid.GenerateUUID()
			if err == nil {
				requestID = generated
			}
		}

		if requestID != "" {
			w.Header().Set("X-Request-Id", requestID)
			ctx := context.WithValue(r.Context(), constants.RequestIDCtx, requestID)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}
