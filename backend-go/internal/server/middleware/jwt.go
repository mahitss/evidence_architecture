package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/mahitss/evidence_architecture/backend-go/internal/auth"
)

type claimsCtxKey struct{}

// JWTVerifier verifies bearer JWT tokens and returns structured Claims.
type JWTVerifier interface {
	Verify(token string) (auth.Claims, error)
}

// RequireAdminJWT extracts an Authorization: Bearer token, verifies it, and injects
// admin claims into the request context. Missing or invalid tokens return 401;
// expired tokens return 403.
func RequireAdminJWT(verifier JWTVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawAuth := strings.TrimSpace(r.Header.Get("Authorization"))
			if rawAuth == "" {
				WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token")
				return
			}

			parts := strings.SplitN(rawAuth, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "invalid authorization header")
				return
			}

			claims, err := verifier.Verify(strings.TrimSpace(parts[1]))
			if err != nil {
				if errors.Is(err, auth.ErrTokenExpired) {
					WriteError(w, r, http.StatusForbidden, "TOKEN_EXPIRED", "token expired")
					return
				}
				WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "invalid bearer token")
				return
			}

			ctx := context.WithValue(r.Context(), claimsCtxKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext retrieves verified admin JWT claims from the request context.
func ClaimsFromContext(ctx context.Context) (auth.Claims, bool) {
	if ctx == nil {
		return auth.Claims{}, false
	}
	c, ok := ctx.Value(claimsCtxKey{}).(auth.Claims)
	return c, ok
}
