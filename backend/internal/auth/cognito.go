package auth

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

var (
	jwksOnce  sync.Once
	jwksCache jwk.Set
	jwksError error
)

// Claims holds validated token claims extracted from a Cognito ID token.
type Claims struct {
	Sub      string
	Username string
	Email    string
}

func jwksURL() string {
	region := os.Getenv("COGNITO_REGION")
	poolID := os.Getenv("COGNITO_USER_POOL_ID")
	return fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json", region, poolID)
}

// fetchJWKS fetches and caches the Cognito JWKS. Called at most once per container.
func fetchJWKS(ctx context.Context) (jwk.Set, error) {
	jwksOnce.Do(func() {
		set, err := jwk.Fetch(ctx, jwksURL())
		if err != nil {
			jwksError = fmt.Errorf("fetch JWKS: %w", err)
			return
		}
		jwksCache = set
	})
	return jwksCache, jwksError
}

// ValidateToken validates a Cognito ID token and returns its claims.
func ValidateToken(ctx context.Context, tokenString string) (*Claims, error) {
	keySet, err := fetchJWKS(ctx)
	if err != nil {
		return nil, err
	}

	clientID := os.Getenv("COGNITO_CLIENT_ID")

	token, err := jwt.Parse(
		[]byte(tokenString),
		jwt.WithKeySet(keySet),
		jwt.WithValidate(true),
		jwt.WithAudience(clientID),
		jwt.WithClaimValue("token_use", "id"),
	)
	if err != nil {
		return nil, fmt.Errorf("parse/validate token: %w", err)
	}

	sub := token.Subject()

	username, _ := token.Get("cognito:username")
	usernameStr, _ := username.(string)
	if usernameStr == "" {
		if nameVal, ok := token.Get("name"); ok {
			usernameStr, _ = nameVal.(string)
		}
	}

	email, _ := token.Get("email")
	emailStr, _ := email.(string)

	return &Claims{
		Sub:      sub,
		Username: usernameStr,
		Email:    emailStr,
	}, nil
}
