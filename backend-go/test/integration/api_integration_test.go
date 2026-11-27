package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/auth"
	"github.com/mahitss/evidence_architecture/backend-go/internal/config"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultIntegrationDBURL = "postgres://postgres@127.0.0.1:5432/truecrime_dev?sslmode=disable"
	seedAdminID             = "00000000-0000-0000-0000-000000000001"
	seedAdminEmail          = "admin-test@truecrime.local"
	seedCaseID              = "10000000-0000-0000-0000-000000000001"
	seedLegalReviewID       = "20000000-0000-0000-0000-000000000001"
)

func TestAPI_IntegrationAgainstRealPostgres(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = defaultIntegrationDBURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	st, err := store.New(ctx, dbURL)
	require.NoError(t, err, "real local PostgreSQL must be running with migrations applied (make services-up && bash scripts/migrate.sh up)")
	t.Cleanup(st.Close)

	// Preserve the exact non-authenticating dev-seed password hash in 000002_seed_dev.up.sql:
	// temporarily assign a freshly generated Argon2id hash to the seed admin row for this test
	// and restore the original hash in t.Cleanup so seed data stays untouched.
	var originalHash string
	err = st.Pool().QueryRow(
		ctx,
		`SELECT password_hash FROM admin_users WHERE id = $1`,
		seedAdminID,
	).Scan(&originalHash)
	require.NoError(t, err, "seed admin row 00000000-0000-0000-0000-000000000001 must exist")

	testPassword := "IntegrationSeedAdminPass!2026"
	ephemeralHash, err := auth.HashPassword(testPassword)
	require.NoError(t, err)

	_, err = st.Pool().Exec(
		ctx,
		`UPDATE admin_users SET password_hash = $1 WHERE id = $2`,
		ephemeralHash,
		seedAdminID,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer restoreCancel()
		_, restoreErr := st.Pool().Exec(
			restoreCtx,
			`UPDATE admin_users SET password_hash = $1 WHERE id = $2`,
			originalHash,
			seedAdminID,
		)
		assert.NoError(t, restoreErr)
	})

	cfg := config.Config{
		DatabaseURL:          dbURL,
		JWTSecret:            "integration-test-jwt-secret-at-least-32-chars!",
		ListenAddr:           "127.0.0.1:0",
		LogLevel:             "error",
		JWTTTL:               60 * time.Minute,
		Port:                 "8080",
		Version:              "0.1.0-integration",
		LoginRateLimitPerMin: 5,
	}

	srv, err := server.New(cfg, st)
	require.NoError(t, err)
	srv.AuthHandler().SetFailureDelay(10 * time.Millisecond)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := ts.Client()

	// 1. GET /health -> 200 {"status":"ok","db":"ok"}
	t.Run("GET /health returns ok", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotEmpty(t, resp.Header.Get("X-Request-ID"))

		var payload map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
		assert.Equal(t, "ok", payload["status"])
		assert.Equal(t, "ok", payload["db"])
		assert.Equal(t, "0.1.0-integration", payload["version"])
	})

	// 2. POST /api/v1/admin/login with seed admin -> JWT
	var adminJWT string
	t.Run("POST /api/v1/admin/login with seed admin returns JWT", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    seedAdminEmail,
			"password": testPassword,
		})
		resp, err := client.Post(ts.URL+"/api/v1/admin/login", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var payload struct {
			Token     string `json:"token"`
			ExpiresAt string `json:"expires_at"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
		assert.NotEmpty(t, payload.Token)
		assert.NotEmpty(t, payload.ExpiresAt)
		adminJWT = payload.Token
	})

	// 3. POST /api/v1/admin/login with wrong password -> 401
	t.Run("POST /api/v1/admin/login with wrong password returns 401", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    seedAdminEmail,
			"password": "definitely-wrong-password",
		})
		resp, err := client.Post(ts.URL+"/api/v1/admin/login", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		var env struct {
			Error struct {
				Code      string `json:"code"`
				Message   string `json:"message"`
				RequestID string `json:"request_id"`
			} `json:"error"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
		assert.Equal(t, "UNAUTHORIZED", env.Error.Code)
		assert.Equal(t, "invalid credentials", env.Error.Message)
		assert.NotEmpty(t, env.Error.RequestID)
	})

	// 4. GET /api/v1/admin/cases/{seed_case_id} with JWT -> 200 + correct case + review summary
	t.Run("GET /api/v1/admin/cases/{seed_case_id} with JWT returns 200 and seed case", func(t *testing.T) {
		require.NotEmpty(t, adminJWT)
		req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/cases/"+seedCaseID, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+adminJWT)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var payload struct {
			ID                   string `json:"id"`
			Slug                 string `json:"slug"`
			Title                string `json:"title"`
			Status               string `json:"status"`
			Category             string `json:"category"`
			PublishedLegalReview *struct {
				ID                        string `json:"id"`
				ExternalCounselRef        string `json:"external_counsel_ref"`
				Decision                  string `json:"decision"`
				ProhibitedChecklistPassed bool   `json:"prohibited_checklist_passed"`
			} `json:"published_legal_review"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
		assert.Equal(t, seedCaseID, payload.ID)
		assert.Equal(t, "test-case-001", payload.Slug)
		assert.Equal(t, "Test Case — Placeholder", payload.Title)
		assert.Equal(t, "DRAFT", payload.Status)
		assert.Equal(t, "financial_crime", payload.Category)
		require.NotNil(t, payload.PublishedLegalReview)
		assert.Equal(t, seedLegalReviewID, payload.PublishedLegalReview.ID)
		assert.Equal(t, "DEV-SEED", payload.PublishedLegalReview.ExternalCounselRef)
		assert.Equal(t, "APPROVED", payload.PublishedLegalReview.Decision)
		assert.True(t, payload.PublishedLegalReview.ProhibitedChecklistPassed)
	})

	// 5. GET /api/v1/admin/cases/{random_uuid} -> 404
	t.Run("GET /api/v1/admin/cases/{random_uuid} returns 404", func(t *testing.T) {
		require.NotEmpty(t, adminJWT)
		req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/cases/"+uuid.NewString(), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+adminJWT)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	// 6. GET /api/v1/admin/cases/{seed_case_id} without JWT -> 401
	t.Run("GET /api/v1/admin/cases/{seed_case_id} without JWT returns 401", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/v1/admin/cases/" + seedCaseID)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	// 7. Rapid login attempts -> rate limited (429)
	t.Run("POST /api/v1/admin/login rapid attempts return 429", func(t *testing.T) {
		// Verify both: (a) rapid 2-attempt cap -> 3rd returns 429 against real router+store
		rateCfg := cfg
		rateCfg.LoginRateLimitPerMin = 2
		rateSrv, err := server.New(rateCfg, st)
		require.NoError(t, err)
		rateSrv.AuthHandler().SetFailureDelay(0)

		rateTS := httptest.NewServer(rateSrv.Handler())
		defer rateTS.Close()

		badBody, _ := json.Marshal(map[string]string{
			"email":    seedAdminEmail,
			"password": "wrong-password-burst",
		})

		for attempt := 1; attempt <= 2; attempt++ {
			resp, err := rateTS.Client().Post(rateTS.URL+"/api/v1/admin/login", "application/json", bytes.NewReader(badBody))
			require.NoError(t, err)
			_ = resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "attempt %d should return 401", attempt)
		}

		resp, err := rateTS.Client().Post(rateTS.URL+"/api/v1/admin/login", "application/json", bytes.NewReader(badBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
		assert.NotEmpty(t, resp.Header.Get("Retry-After"))
	})
}
