package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mahitss/evidence_architecture/backend-go/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubVerifier struct {
	claims auth.Claims
	err    error
}

func (s stubVerifier) Verify(_ string) (auth.Claims, error) {
	return s.claims, s.err
}

func TestRequireAdminJWT(t *testing.T) {
	t.Parallel()

	mgr, err := auth.NewJWTManager("middleware-test-jwt-secret-32-chars-minimum")
	require.NoError(t, err)

	validToken, err := mgr.Issue("00000000-0000-0000-0000-000000000001", time.Hour)
	require.NoError(t, err)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Subject", claims.Subject)
		w.WriteHeader(http.StatusOK)
	})

	t.Run("missing authorization header returns 401", func(t *testing.T) {
		t.Parallel()
		h := RequestID(RequireAdminJWT(mgr)(nextHandler))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/cases/123", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		var env ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		assert.Equal(t, "UNAUTHORIZED", env.Error.Code)
		assert.NotEmpty(t, env.Error.RequestID)
		assert.Equal(t, rec.Header().Get(HeaderRequestID), env.Error.RequestID)
	})

	t.Run("invalid bearer token returns 401", func(t *testing.T) {
		t.Parallel()
		h := RequestID(RequireAdminJWT(mgr)(nextHandler))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/cases/123", nil)
		req.Header.Set("Authorization", "Bearer not.a.valid.jwt")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		var env ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		assert.Equal(t, "UNAUTHORIZED", env.Error.Code)
		assert.NotEmpty(t, env.Error.RequestID)
	})

	t.Run("expired bearer token returns 403", func(t *testing.T) {
		t.Parallel()
		expiredVerifier := stubVerifier{err: auth.ErrTokenExpired}
		h := RequestID(RequireAdminJWT(expiredVerifier)(nextHandler))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/cases/123", nil)
		req.Header.Set("Authorization", "Bearer expired.token.value")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		var env ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		assert.Equal(t, "TOKEN_EXPIRED", env.Error.Code)
		assert.NotEmpty(t, env.Error.RequestID)
	})

	t.Run("valid bearer token injects claims and passes through", func(t *testing.T) {
		t.Parallel()
		h := RequestID(RequireAdminJWT(mgr)(nextHandler))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/cases/123", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "00000000-0000-0000-0000-000000000001", rec.Header().Get("X-Subject"))
		assert.NotEmpty(t, rec.Header().Get(HeaderRequestID))
	})
}
