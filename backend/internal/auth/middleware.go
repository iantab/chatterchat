package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const ClaimsKey contextKey = "claims"

// Middleware validates the Bearer token and injects Claims into the request context.
// Returns 401 if the token is missing or invalid.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := ValidateToken(r.Context(), tokenStr)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromContext retrieves Claims injected by Middleware.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(ClaimsKey).(*Claims)
	return c, ok
}
