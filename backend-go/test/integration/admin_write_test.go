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
	"github.com/mahitss/evidence_architecture/backend-go/internal/storage"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminWriteWorkflow_EndToEndAgainstRealPostgresAndRedis(t *testing.T) {
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
	require.NoError(t, err, "local Postgres must be up with migrations applied")
	t.Cleanup(st.Close)

	pub, err := queue.NewPublisher(ctx, redisURL)
	require.NoError(t, err, "local Redis must be up on 127.0.0.1:6379")
	t.Cleanup(func() { _ = pub.Close() })

	storageRoot := t.TempDir()
	localStorage, err := storage.NewLocalStorage(storageRoot)
	require.NoError(t, err)

	// Temporarily assign an authenticatable Argon2id hash to the seed admin row and guarantee
	// restoration via t.Cleanup (runs in LIFO order even on panic, before st.Close()).
	var originalHash string
	err = st.Pool().QueryRow(
		ctx,
		`SELECT password_hash FROM admin_users WHERE id = $1`,
		seedAdminID,
	).Scan(&originalHash)
	require.NoError(t, err)

	testPassword := "WriteFlowSeedAdminPass!2026"
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
		RedisURL:             redisURL,
		LocalStorageRoot:     storageRoot,
		JWTSecret:            "write-integration-test-jwt-secret-min-32-chars!",
		ListenAddr:           "127.0.0.1:0",
		LogLevel:             "error",
		JWTTTL:               60 * time.Minute,
		Port:                 "8080",
		Version:              "0.1.0-integration",
		LoginRateLimitPerMin: 20,
	}

	srv, err := server.NewWithDeps(cfg, st, localStorage, pub)
	require.NoError(t, err)
	srv.AuthHandler().SetFailureDelay(0)

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	client := ts.Client()

	// Track created case ID for clean database teardown (note: audit_log is append-only so rows stay).
	var createdCaseID string
	t.Cleanup(func() {
		if createdCaseID == "" {
			return
		}
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_, _ = st.Pool().Exec(cleanCtx, `UPDATE cases SET published_legal_review_id = NULL WHERE id = $1`, createdCaseID)
		_, _ = st.Pool().Exec(cleanCtx, `DELETE FROM evidence_items WHERE case_id = $1`, createdCaseID)
		_, _ = st.Pool().Exec(cleanCtx, `DELETE FROM legal_review_records WHERE case_id = $1`, createdCaseID)
		_, _ = st.Pool().Exec(cleanCtx, `DELETE FROM cases WHERE id = $1`, createdCaseID)
	})

	// 1. Login as seed admin -> JWT
	loginBody, _ := json.Marshal(map[string]string{
		"email":    seedAdminEmail,
		"password": testPassword,
	})
	loginResp, err := client.Post(ts.URL+"/api/v1/admin/login", "application/json", bytes.NewReader(loginBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, loginResp.StatusCode)
	var loginPayload struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.NewDecoder(loginResp.Body).Decode(&loginPayload))
	_ = loginResp.Body.Close()
	require.NotEmpty(t, loginPayload.Token)
	authHeader := "Bearer " + loginPayload.Token

	// 2. Create case (with a Genuine Gaps entry required by DB publish trigger)
	uniqueSlug := "write-flow-" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	createBody := fmt.Sprintf(`{
		"slug": %q,
		"title": "Integration Write-Flow Case",
		"summary": "Initial DOJ filing summary",
		"category": "financial_crime",
		"genuine_gaps": [{"claim":"Offshore beneficiary list","why_unverified":"Exhibit remains under seal","highest_available_tier":"None"}]
	}`, uniqueSlug)

	createReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/cases", strings.NewReader(createBody))
	require.NoError(t, err)
	createReq.Header.Set("Authorization", authHeader)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := client.Do(createReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createdCase struct {
		ID     string `json:"id"`
		Slug   string `json:"slug"`
		Status string `json:"status"`
	}
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&createdCase))
	_ = createResp.Body.Close()
	require.NotEmpty(t, createdCase.ID)
	assert.Equal(t, uniqueSlug, createdCase.Slug)
	assert.Equal(t, "DRAFT", createdCase.Status)
	createdCaseID = createdCase.ID

	// 2b. Verify duplicate slug returns 409 Conflict
	dupReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/cases", strings.NewReader(createBody))
	dupReq.Header.Set("Authorization", authHeader)
	dupReq.Header.Set("Content-Type", "application/json")
	dupResp, err := client.Do(dupReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, dupResp.StatusCode)
	_ = dupResp.Body.Close()

	// 2c. Update DRAFT case -> 200
	updReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/admin/cases/"+createdCaseID, strings.NewReader(`{"title":"Integration Write-Flow Case Updated"}`))
	updReq.Header.Set("Authorization", authHeader)
	updReq.Header.Set("Content-Type", "application/json")
	updResp, err := client.Do(updReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, updResp.StatusCode)
	_ = updResp.Body.Close()

	// 3. Verify Q5 rejection at HTTP handler layer (400) before DB hit
	q5Resp := postMultipartEvidence(t, client, ts.URL+"/api/v1/admin/cases/"+createdCaseID+"/evidence", authHeader, map[string]string{
		"title":             "Blocked Third-Party Public Domain",
		"source_type":       "third_party_exhibit",
		"license_status":    "PUBLIC_DOMAIN",
		"verification_tier": "1",
		"reported_as_date":  "2026-01-15",
	}, []byte("%PDF-1.4\nq5\n"), "application/pdf")
	assert.Equal(t, http.StatusBadRequest, q5Resp.StatusCode)
	_ = q5Resp.Body.Close()

	// 4. Upload valid evidence -> 202 Accepted, PENDING, file stored, Redis stream entry exists
	streamBefore, err := pub.Client().XLen(ctx, queue.EvidenceProcessStream).Result()
	require.NoError(t, err)

	uploadResp := postMultipartEvidence(t, client, ts.URL+"/api/v1/admin/cases/"+createdCaseID+"/evidence", authHeader, map[string]string{
		"title":             "S.D.N.Y. Criminal Indictment",
		"description":       "U.S. v. Example, 26-cr-0001 ECF No. 1",
		"source_type":       "gov_primary_record",
		"license_status":    "PUBLIC_DOMAIN",
		"verification_tier": "1",
		"reported_as_date":  "2026-01-15",
	}, []byte("%PDF-1.4\nindictment payload for integration test\n"), "application/pdf")
	require.Equal(t, http.StatusAccepted, uploadResp.StatusCode)

	var uploadPayload struct {
		EvidenceID       string `json:"evidence_id"`
		ProcessingStatus string `json:"processing_status"`
		JobPublished     bool   `json:"job_published"`
		SHA256Hash       string `json:"sha256_hash"`
		StorageKey       string `json:"storage_key"`
	}
	require.NoError(t, json.NewDecoder(uploadResp.Body).Decode(&uploadPayload))
	_ = uploadResp.Body.Close()
	assert.NotEmpty(t, uploadPayload.EvidenceID)
	assert.Equal(t, "PENDING", uploadPayload.ProcessingStatus)
	assert.True(t, uploadPayload.JobPublished)
	assert.Len(t, uploadPayload.SHA256Hash, 64)

	// Verify Redis Streams 'evidence.process' received the published entry
	streamAfter, err := pub.Client().XLen(ctx, queue.EvidenceProcessStream).Result()
	require.NoError(t, err)
	assert.Greater(t, streamAfter, streamBefore, "Redis stream evidence.process must contain newly published entry")

	// Verify GET /api/v1/admin/cases/{id} returns full evidence_items list with signed_url
	getReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/cases/"+createdCaseID, nil)
	getReq.Header.Set("Authorization", authHeader)
	getResp, err := client.Do(getReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, getResp.StatusCode)
	var caseDetail struct {
		ID            string `json:"id"`
		EvidenceItems []struct {
			ID        string `json:"id"`
			SignedURL string `json:"signed_url"`
		} `json:"evidence_items"`
	}
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&caseDetail))
	_ = getResp.Body.Close()
	require.Len(t, caseDetail.EvidenceItems, 1)
	assert.Equal(t, uploadPayload.EvidenceID, caseDetail.EvidenceItems[0].ID)
	assert.True(t, strings.HasPrefix(caseDetail.EvidenceItems[0].SignedURL, "file://"))

	// 5. Attempt publish BEFORE legal review -> 422 with exact PostgreSQL trigger message
	pubNoRevReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/cases/"+createdCaseID+"/publish", nil)
	pubNoRevReq.Header.Set("Authorization", authHeader)
	pubNoRevResp, err := client.Do(pubNoRevReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, pubNoRevResp.StatusCode)
	var pubNoRevErr struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(pubNoRevResp.Body).Decode(&pubNoRevErr))
	_ = pubNoRevResp.Body.Close()
	assert.Contains(t, pubNoRevErr.Error.Message, "published_legal_review_id is required")

	// Mark evidence cleared via PUT /evidence/{id} and simulate Rust processor completion
	// (Rust worker is Prompt 8d; DB trigger enforces processing_status='COMPLETED' & prohibited_content_cleared=TRUE).
	clearReq, _ := http.NewRequest(
		http.MethodPut,
		ts.URL+"/api/v1/admin/evidence/"+uploadPayload.EvidenceID,
		strings.NewReader(`{"prohibited_content_cleared":true}`),
	)
	clearReq.Header.Set("Authorization", authHeader)
	clearReq.Header.Set("Content-Type", "application/json")
	clearResp, err := client.Do(clearReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, clearResp.StatusCode)
	_ = clearResp.Body.Close()

	_, err = st.Pool().Exec(ctx, `UPDATE evidence_items SET processing_status = 'COMPLETED' WHERE id = $1`, uploadPayload.EvidenceID)
	require.NoError(t, err)

	// 6. Record APPROVED legal review -> 201
	revBody := `{
		"decision": "APPROVED",
		"external_counsel_ref": "COUNSEL-INT-8C-001",
		"prohibited_checklist_passed": true,
		"defamation_check_passed": true,
		"privacy_check_passed": true,
		"copyright_check_passed": true,
		"notes": "Integration test legal sign-off"
	}`
	revReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/cases/"+createdCaseID+"/legal-review", strings.NewReader(revBody))
	revReq.Header.Set("Authorization", authHeader)
	revReq.Header.Set("Content-Type", "application/json")
	revResp, err := client.Do(revReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, revResp.StatusCode)
	_ = revResp.Body.Close()

	// 7. Publish case -> 200 PUBLISHED
	pubReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/cases/"+createdCaseID+"/publish", nil)
	pubReq.Header.Set("Authorization", authHeader)
	pubResp, err := client.Do(pubReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, pubResp.StatusCode)
	var publishedCase struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.NewDecoder(pubResp.Body).Decode(&publishedCase))
	_ = pubResp.Body.Close()
	assert.Equal(t, "PUBLISHED", publishedCase.Status)

	// 7b. Attempting PUT /cases/{id} on PUBLISHED case must return 409
	editPubReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/admin/cases/"+createdCaseID, strings.NewReader(`{"title":"Should Fail"}`))
	editPubReq.Header.Set("Authorization", authHeader)
	editPubReq.Header.Set("Content-Type", "application/json")
	editPubResp, err := client.Do(editPubReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, editPubResp.StatusCode)
	_ = editPubResp.Body.Close()

	// 8. Updating evidence metadata on a PUBLISHED case MUST force case status back to IN_REVIEW
	evMutReq, _ := http.NewRequest(
		http.MethodPut,
		ts.URL+"/api/v1/admin/evidence/"+uploadPayload.EvidenceID,
		strings.NewReader(`{"title":"S.D.N.Y. Criminal Indictment (Edited Citation)"}`),
	)
	evMutReq.Header.Set("Authorization", authHeader)
	evMutReq.Header.Set("Content-Type", "application/json")
	evMutResp, err := client.Do(evMutReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, evMutResp.StatusCode)
	var evMutPayload struct {
		ReReviewTriggered bool   `json:"re_review_triggered"`
		ParentCaseStatus  string `json:"parent_case_status"`
	}
	require.NoError(t, json.NewDecoder(evMutResp.Body).Decode(&evMutPayload))
	_ = evMutResp.Body.Close()
	assert.True(t, evMutPayload.ReReviewTriggered)
	assert.Equal(t, "IN_REVIEW", evMutPayload.ParentCaseStatus)

	// Confirm in Postgres that status is IN_REVIEW and published_legal_review_id was cleared
	var dbStatus string
	var dbReviewID *string
	err = st.Pool().QueryRow(ctx, `SELECT status, published_legal_review_id::text FROM cases WHERE id = $1`, createdCaseID).Scan(&dbStatus, &dbReviewID)
	require.NoError(t, err)
	assert.Equal(t, "IN_REVIEW", dbStatus)
	assert.Nil(t, dbReviewID, "updating evidence on PUBLISHED case must require a new legal_review_records row before re-publish")

	// 9. Record a new APPROVED legal review -> re-publish -> retract -> verify RETRACTED
	revReq2, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/cases/"+createdCaseID+"/legal-review", strings.NewReader(revBody))
	revReq2.Header.Set("Authorization", authHeader)
	revReq2.Header.Set("Content-Type", "application/json")
	revResp2, err := client.Do(revReq2)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, revResp2.StatusCode)
	_ = revResp2.Body.Close()

	pubReq2, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/cases/"+createdCaseID+"/publish", nil)
	pubReq2.Header.Set("Authorization", authHeader)
	pubResp2, err := client.Do(pubReq2)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, pubResp2.StatusCode)
	_ = pubResp2.Body.Close()

	retReq, _ := http.NewRequest(
		http.MethodPost,
		ts.URL+"/api/v1/admin/cases/"+createdCaseID+"/retract",
		strings.NewReader(`{"reason":"Post-publication legal review retraction"}`),
	)
	retReq.Header.Set("Authorization", authHeader)
	retReq.Header.Set("Content-Type", "application/json")
	retResp, err := client.Do(retReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, retResp.StatusCode)
	var retractedCase struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.NewDecoder(retResp.Body).Decode(&retractedCase))
	_ = retResp.Body.Close()
	assert.Equal(t, "RETRACTED", retractedCase.Status)

	// 10. Verify audit_log contains entries for EVERY mutation executed in this workflow
	expectedActions := []struct {
		action     string
		entityID   string
		minMatches int
	}{
		{"case.created", createdCaseID, 1},
		{"case.updated", createdCaseID, 1},
		{"evidence.created", uploadPayload.EvidenceID, 1},
		{"evidence.updated", uploadPayload.EvidenceID, 2},
		{"case.published", createdCaseID, 2},
		{"case.retracted", createdCaseID, 1},
	}
	for _, exp := range expectedActions {
		var count int
		err := st.Pool().QueryRow(
			ctx,
			`SELECT COUNT(*) FROM audit_log WHERE action = $1 AND entity_id = $2`,
			exp.action,
			exp.entityID,
		).Scan(&count)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, exp.minMatches, "audit_log must record action %q for entity %s", exp.action, exp.entityID)
	}

	var legalReviewAuditCount int
	err = st.Pool().QueryRow(
		ctx,
		`SELECT COUNT(*) FROM audit_log WHERE action = 'legal_review.created' AND metadata->>'case_id' = $1`,
		createdCaseID,
	).Scan(&legalReviewAuditCount)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, legalReviewAuditCount, 2, "audit_log must record legal_review.created entries")
}

func postMultipartEvidence(t *testing.T, client *http.Client, url, authHeader string, fields map[string]string, fileBytes []byte, contentType string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		require.NoError(t, mw.WriteField(k, v))
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="exhibit.pdf"`)
	h.Set("Content-Type", contentType)
	part, err := mw.CreatePart(h)
	require.NoError(t, err)
	_, err = part.Write(fileBytes)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	require.NoError(t, err)
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}
