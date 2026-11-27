package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/auth"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAdminUserReader struct {
	users map[string]*models.AdminUser
}

func (m mockAdminUserReader) GetByEmail(_ context.Context, email string) (*models.AdminUser, error) {
	u, ok := m.users[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return nil, store.ErrNotFound
	}
	return u, nil
}

func TestAuthHandler_LoginFlowAndRateLimit(t *testing.T) {
	t.Parallel()

	seedAdminID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	seedEmail := "admin-test@truecrime.local"
	seedPassword := "StrongTestAdminPass!2026"

	hash, err := auth.HashPassword(seedPassword)
	require.NoError(t, err)

	jwtMgr, err := auth.NewJWTManager("handler-auth-test-secret-with-32-plus-bytes!")
	require.NoError(t, err)

	users := mockAdminUserReader{
		users: map[string]*models.AdminUser{
			seedEmail: {
				ID:           seedAdminID,
				Email:        seedEmail,
				PasswordHash: hash,
				Role:         "ADMIN",
				IsActive:     true,
			},
		},
	}

	t.Run("login success with seed admin returns JWT and expires_at without logging raw email", func(t *testing.T) {
		t.Parallel()
		var logBuf bytes.Buffer
		logger := zerolog.New(&logBuf)
		h := NewAuthHandler(users, jwtMgr, time.Hour, 5, logger)
		h.SetFailureDelay(5 * time.Millisecond)

		handler := middleware.RequestID(http.HandlerFunc(h.Login))
		body := `{"email":"admin-test@truecrime.local","password":"StrongTestAdminPass!2026"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:41001"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp loginResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.NotEmpty(t, resp.Token)
		assert.NotEmpty(t, resp.ExpiresAt)

		claims, verifyErr := jwtMgr.Verify(resp.Token)
		require.NoError(t, verifyErr)
		assert.Equal(t, seedAdminID.String(), claims.Subject)

		// Verify email is hashed in logs and never logged in plaintext
		logLine := logBuf.String()
		assert.NotContains(t, logLine, seedEmail)
		assert.Contains(t, logLine, hashEmailForLog(seedEmail))
		assert.Contains(t, logLine, `"success":true`)
	})

	t.Run("wrong password returns generic 401 invalid credentials", func(t *testing.T) {
		t.Parallel()
		var logBuf bytes.Buffer
		h := NewAuthHandler(users, jwtMgr, time.Hour, 5, zerolog.New(&logBuf))
		h.SetFailureDelay(10 * time.Millisecond)

		handler := middleware.RequestID(http.HandlerFunc(h.Login))
		body := `{"email":"admin-test@truecrime.local","password":"wrong-password"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:41002"
		rec := httptest.NewRecorder()

		start := time.Now()
		handler.ServeHTTP(rec, req)
		elapsed := time.Since(start)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.GreaterOrEqual(t, elapsed, 10*time.Millisecond)

		var env middleware.ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		assert.Equal(t, "UNAUTHORIZED", env.Error.Code)
		assert.Equal(t, "invalid credentials", env.Error.Message)
		assert.NotEmpty(t, env.Error.RequestID)
		assert.NotContains(t, logBuf.String(), seedEmail)
	})

	t.Run("unknown email returns identical generic 401 invalid credentials", func(t *testing.T) {
		t.Parallel()
		h := NewAuthHandler(users, jwtMgr, time.Hour, 5, zerolog.Nop())
		h.SetFailureDelay(10 * time.Millisecond)

		handler := middleware.RequestID(http.HandlerFunc(h.Login))
		body := `{"email":"nonexistent@truecrime.local","password":"some-password"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:41003"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		var env middleware.ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		assert.Equal(t, "UNAUTHORIZED", env.Error.Code)
		assert.Equal(t, "invalid credentials", env.Error.Message)
	})

	t.Run("rate limit returns 429 after 5 attempts per minute from same IP", func(t *testing.T) {
		t.Parallel()
		h := NewAuthHandler(users, jwtMgr, time.Hour, 5, zerolog.Nop())
		h.SetFailureDelay(0)

		handler := middleware.RequestID(http.HandlerFunc(h.Login))
		body := `{"email":"unknown@truecrime.local","password":"bad"}`

		for i := 1; i <= 5; i++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(body))
			req.RemoteAddr = "198.51.100.10:5000"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusUnauthorized, rec.Code, "attempt %d should be 401 before rate limit", i)
		}

		// 6th request from the same IP within 1 minute must be rate limited (429)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(body))
		req.RemoteAddr = "198.51.100.10:5001"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		assert.NotEmpty(t, rec.Header().Get("Retry-After"))
		var env middleware.ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		assert.Equal(t, "RATE_LIMITED", env.Error.Code)
	})
}
