package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/auth"
	"github.com/mahitss/evidence_architecture/backend-go/internal/config"
	"github.com/mahitss/evidence_architecture/backend-go/internal/queue"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/storage"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdempotencyMiddleware_EvidenceUploadAgainstRealPostgresAndRedis(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = defaultIntegrationDBURL
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = queue.DefaultRedisURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	st, err := store.New(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(st.Close)

	pub, err := queue.NewPublisher(ctx, redisURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	storageRoot := t.TempDir()
	localStorage, err := storage.NewLocalStorage(storageRoot)
	require.NoError(t, err)

	// LIFO t.Cleanup guarantees seed admin password hash restoration before st.Close().
	var originalHash string
	err = st.Pool().QueryRow(ctx, `SELECT password_hash FROM admin_users WHERE id = $1`, seedAdminID).Scan(&originalHash)
	require.NoError(t, err)

	testPassword := "IdemSeedAdminPass!2026"
	ephemeralHash, err := auth.HashPassword(testPassword)
	require.NoError(t, err)
	_, err = st.Pool().Exec(ctx, `UPDATE admin_users SET password_hash = $1 WHERE id = $2`, ephemeralHash, seedAdminID)
	require.NoError(t, err)
	t.Cleanup(func() {
		restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer restoreCancel()
		_, restoreErr := st.Pool().Exec(restoreCtx, `UPDATE admin_users SET password_hash = $1 WHERE id = $2`, originalHash, seedAdminID)
		assert.NoError(t, restoreErr)
	})

	cfg := config.Config{
		DatabaseURL:          dbURL,
		RedisURL:             redisURL,
		LocalStorageRoot:     storageRoot,
		JWTSecret:            "idempotency-integration-test-jwt-secret-32chars!",
		ListenAddr:           "127.0.0.1:0",
		LogLevel:             "error",
		JWTTTL:               60 * time.Minute,
		Port:                 "8080",
		Version:              "0.1.0-idem-int",
		LoginRateLimitPerMin: 20,
	}

	srv, err := server.NewWithDeps(cfg, st, localStorage, pub)
	require.NoError(t, err)
	srv.AuthHandler().SetFailureDelay(0)

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	client := ts.Client()

	// Login -> Bearer JWT (login does NOT require Idempotency-Key)
	loginBody, _ := json.Marshal(map[string]string{"email": seedAdminEmail, "password": testPassword})
	loginResp, err := client.Post(ts.URL+"/api/v1/admin/login", "application/json", bytes.NewReader(loginBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, loginResp.StatusCode)
	var loginPayload struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.NewDecoder(loginResp.Body).Decode(&loginPayload))
	_ = loginResp.Body.Close()
	authHeader := "Bearer " + loginPayload.Token

	// Create test case
	uniqueSlug := "idem-case-" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	caseIdemKey := "case_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	t.Cleanup(func() {
		_ = pub.Client().Del(context.Background(), middleware.IdempotencyKeyPrefix+caseIdemKey).Err()
	})

	createReq, err := http.NewRequest(
		http.MethodPost,
		ts.URL+"/api/v1/admin/cases",
		strings.NewReader(fmt.Sprintf(`{"slug":%q,"title":"Idempotency Evidence Case","category":"financial_crime"}`, uniqueSlug)),
	)
	require.NoError(t, err)
	createReq.Header.Set("Authorization", authHeader)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Idempotency-Key", caseIdemKey)
	createResp, err := client.Do(createReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createdCase struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&createdCase))
	_ = createResp.Body.Close()
	require.NotEmpty(t, createdCase.ID)

	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_, _ = st.Pool().Exec(cleanCtx, `DELETE FROM evidence_items WHERE case_id = $1`, createdCase.ID)
		_, _ = st.Pool().Exec(cleanCtx, `DELETE FROM cases WHERE id = $1`, createdCase.ID)
	})

	// Build a deterministic multipart body with a fixed boundary so identical retries have identical SHA-256.
	buildFixedMultipart := func(title string, fileBytes []byte) ([]byte, string) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		require.NoError(t, mw.SetBoundary("FIXED_IDEM_BOUNDARY_2026"))
		require.NoError(t, mw.WriteField("title", title))
		require.NoError(t, mw.WriteField("source_type", "gov_primary_record"))
		require.NoError(t, mw.WriteField("license_status", "PUBLIC_DOMAIN"))
		require.NoError(t, mw.WriteField("verification_tier", "1"))
		require.NoError(t, mw.WriteField("reported_as_date", "2026-01-15"))
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="file"; filename="filing.pdf"`)
		h.Set("Content-Type", "application/pdf")
		part, err := mw.CreatePart(h)
		require.NoError(t, err)
		_, err = part.Write(fileBytes)
		require.NoError(t, err)
		require.NoError(t, mw.Close())
		return buf.Bytes(), mw.FormDataContentType()
	}

	evIdemKey := "ev_idem_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	t.Cleanup(func() {
		_ = pub.Client().Del(context.Background(), middleware.IdempotencyKeyPrefix+evIdemKey).Err()
	})

	payloadA, contentTypeA := buildFixedMultipart("DOJ Exhibit One", []byte("%PDF-1.4\nidempotency test exhibit\n"))
	uploadURL := ts.URL + "/api/v1/admin/cases/" + createdCase.ID + "/evidence"

	// 1. First POST /evidence -> 202 Accepted
	req1, err := http.NewRequest(http.MethodPost, uploadURL, bytes.NewReader(payloadA))
	require.NoError(t, err)
	req1.Header.Set("Authorization", authHeader)
	req1.Header.Set("Content-Type", contentTypeA)
	req1.Header.Set("Idempotency-Key", evIdemKey)
	resp1, err := client.Do(req1)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, resp1.StatusCode)
	var body1 struct {
		EvidenceID       string `json:"evidence_id"`
		ProcessingStatus string `json:"processing_status"`
		JobPublished     bool   `json:"job_published"`
		SHA256Hash       string `json:"sha256_hash"`
		StorageKey       string `json:"storage_key"`
	}
	require.NoError(t, json.NewDecoder(resp1.Body).Decode(&body1))
	_ = resp1.Body.Close()
	require.NotEmpty(t, body1.EvidenceID)

	// 2. Second POST /evidence with SAME Idempotency-Key and SAME body -> cached 202, identical evidence_id, no duplicate row
	req2, err := http.NewRequest(http.MethodPost, uploadURL, bytes.NewReader(payloadA))
	require.NoError(t, err)
	req2.Header.Set("Authorization", authHeader)
	req2.Header.Set("Content-Type", contentTypeA)
	req2.Header.Set("Idempotency-Key", evIdemKey)
	resp2, err := client.Do(req2)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, resp2.StatusCode)
	assert.Equal(t, "true", resp2.Header.Get("X-Idempotent-Replayed"))
	var body2 struct {
		EvidenceID       string `json:"evidence_id"`
		ProcessingStatus string `json:"processing_status"`
		JobPublished     bool   `json:"job_published"`
		SHA256Hash       string `json:"sha256_hash"`
		StorageKey       string `json:"storage_key"`
	}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&body2))
	_ = resp2.Body.Close()
	assert.Equal(t, body1, body2, "idempotent replay must return identical 202 payload")

	// Confirm only 1 evidence_items row exists for this case
	var evCount int
	err = st.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM evidence_items WHERE case_id = $1`, createdCase.ID).Scan(&evCount)
	require.NoError(t, err)
	assert.Equal(t, 1, evCount, "duplicate idempotent upload must NOT create a second evidence_items row")

	// Confirm audit_log has EXACTLY ONE 'evidence.created' entry despite two requests
	var auditCount int
	err = st.Pool().QueryRow(
		ctx,
		`SELECT COUNT(*) FROM audit_log WHERE action = 'evidence.created' AND entity_id = $1`,
		body1.EvidenceID,
	).Scan(&auditCount)
	require.NoError(t, err)
	assert.Equal(t, 1, auditCount, "audit_log must have exactly one evidence.created entry despite two requests")

	// 3. Third POST /evidence with SAME Idempotency-Key but DIFFERENT body -> 409 IDEMPOTENCY_CONFLICT
	payloadB, contentTypeB := buildFixedMultipart("DOJ Exhibit TWO (Different)", []byte("%PDF-1.4\ndifferent bytes\n"))
	req3, err := http.NewRequest(http.MethodPost, uploadURL, bytes.NewReader(payloadB))
	require.NoError(t, err)
	req3.Header.Set("Authorization", authHeader)
	req3.Header.Set("Content-Type", contentTypeB)
	req3.Header.Set("Idempotency-Key", evIdemKey)
	resp3, err := client.Do(req3)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, resp3.StatusCode)
	var conflictEnv middleware.ErrorEnvelopeResponse
	require.NoError(t, json.NewDecoder(resp3.Body).Decode(&conflictEnv))
	_ = resp3.Body.Close()
	assert.Equal(t, "IDEMPOTENCY_CONFLICT", conflictEnv.Error.Code)
	assert.Equal(t, "Idempotency-Key reused with different body", conflictEnv.Error.Message)

	// Confirm still only 1 evidence row
	err = st.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM evidence_items WHERE case_id = $1`, createdCase.ID).Scan(&evCount)
	require.NoError(t, err)
	assert.Equal(t, 1, evCount)
}
