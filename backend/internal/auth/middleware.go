package auth

import (
	"context"
	"net/http"
	"os"
	"strings"
)

type contextKey string

const ClaimsKey contextKey = "claims"

// Middleware validates the Bearer token and injects Claims into the request context.
// Returns 401 if the token is missing or invalid.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if devUser := os.Getenv("LOCAL_DEV_USER"); devUser != "" {
			parts := strings.SplitN(devUser, ":", 3)
			claims := &Claims{Sub: parts[0]}
			if len(parts) > 1 {
				claims.Username = parts[1]
			}
			if len(parts) > 2 {
				claims.Email = parts[2]
			}
			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

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
