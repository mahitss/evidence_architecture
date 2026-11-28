package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/auth"
	"github.com/mahitss/evidence_architecture/backend-go/internal/config"
	"github.com/mahitss/evidence_architecture/backend-go/internal/queue"
	"github.com/mahitss/evidence_architecture/backend-go/internal/search"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/storage"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const defaultIntegrationMeiliURL = "http://127.0.0.1:7700"

func TestReaderFlow_EndToEndAgainstRealPostgresRedisMeilisearchAndProcessor(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = defaultIntegrationDBURL
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = queue.DefaultRedisURL
	}
	meiliURL := os.Getenv("MEILI_URL")
	if meiliURL == "" {
		meiliURL = defaultIntegrationMeiliURL
	}
	meiliAPIKey := os.Getenv("MEILI_API_KEY")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1. Connect to real Postgres, Redis, and Meilisearch (isolated index for this test run).
	st, err := store.New(ctx, dbURL)
	require.NoError(t, err, "local Postgres must be up with migrations applied")
	t.Cleanup(st.Close)

	pub, err := queue.NewPublisher(ctx, redisURL)
	require.NoError(t, err, "local Redis must be up on 127.0.0.1:6379")
	t.Cleanup(func() { _ = pub.Close() })

	testIndexName := "cases_int_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	meiliClient, err := search.NewClient(ctx, meiliURL, meiliAPIKey, testIndexName, search.StoreDataSource{
		GetCase:      st.Cases.GetByID,
		ListEvidence: st.Evidence.ListByCaseID,
	})
	require.NoError(t, err, "local Meilisearch must be up on 127.0.0.1:7700")
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_ = meiliClient.DeleteIndex(cleanCtx)
	})

	storageRoot := t.TempDir()
	localStorage, err := storage.NewLocalStorage(storageRoot)
	require.NoError(t, err)

	// 2. Start the real Rust processor worker against this storage root, Postgres, Redis, and Meilisearch index.
	stopProcessor := startRustProcessor(t, dbURL, redisURL, storageRoot, meiliURL, meiliAPIKey, testIndexName)
	t.Cleanup(stopProcessor)

	// 3. Temporarily assign an authenticatable Argon2id hash to the seed admin row and guarantee restoration.
	var originalHash string
	err = st.Pool().QueryRow(ctx, `SELECT password_hash FROM admin_users WHERE id = $1`, seedAdminID).Scan(&originalHash)
	require.NoError(t, err)

	testPassword := "ReaderFlowSeedAdmin!2026"
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

	// Clean up synthetic Redis rate-limit keys created by this test.
	searchTestIP := "198.51.100.42"
	corrTestIP := "198.51.100.77"
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_ = pub.Client().Del(cleanCtx,
			"rl:search:"+searchTestIP,
			"rl:corrections:"+corrTestIP,
			"rl:cases_list:127.0.0.1",
			"rl:case_detail:127.0.0.1",
			"rl:evidence_detail:127.0.0.1",
			"rl:search:127.0.0.1",
		).Err()
	})

	cfg := config.Config{
		DatabaseURL:          dbURL,
		RedisURL:             redisURL,
		MeiliURL:             meiliURL,
		MeiliAPIKey:          meiliAPIKey,
		LocalStorageRoot:     storageRoot,
		JWTSecret:            "reader-integration-test-jwt-secret-min-32-chars!",
		ListenAddr:           "127.0.0.1:0",
		LogLevel:             "error",
		JWTTTL:               60 * time.Minute,
		Port:                 "8080",
		Version:              "0.1.0-reader-int",
		LoginRateLimitPerMin: 20,
	}

	idemStore := middleware.NewRedisIdempotencyStore(pub.Client())
	rateLimiter := middleware.NewRedisRateLimiter(pub.Client())

	srv, err := server.NewWithAllDeps(cfg, st, localStorage, pub, idemStore, meiliClient, rateLimiter)
	require.NoError(t, err)
	srv.AuthHandler().SetFailureDelay(0)

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	client := ts.Client()

	var createdCaseID string
	t.Cleanup(func() {
		if createdCaseID == "" {
			return
		}
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_, _ = st.Pool().Exec(cleanCtx, `DELETE FROM correction_requests WHERE case_id = $1`, createdCaseID)
		_, _ = st.Pool().Exec(cleanCtx, `UPDATE cases SET published_legal_review_id = NULL WHERE id = $1`, createdCaseID)
		_, _ = st.Pool().Exec(cleanCtx, `DELETE FROM evidence_items WHERE case_id = $1`, createdCaseID)
		_, _ = st.Pool().Exec(cleanCtx, `DELETE FROM legal_review_records WHERE case_id = $1`, createdCaseID)
		_, _ = st.Pool().Exec(cleanCtx, `DELETE FROM cases WHERE id = $1`, createdCaseID)
	})

	newIdemKey := func() string {
		return "rdr_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}

	// 4. Admin login -> JWT
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
	authHeader := "Bearer " + loginPayload.Token

	// 5. Create DRAFT case
	uniqueSlug := "meridian-bond-fraud-" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	createBody := fmt.Sprintf(`{
		"slug": %q,
		"title": "Meridian Municipal Bond Enforcement Action",
		"summary": "DOJ indictment and parallel SEC enforcement filing.",
		"category": "financial_crime",
		"genuine_gaps": [{"claim":"Undisclosed offshore nominee names","why_unverified":"Sealed grand jury exhibit","highest_available_tier":"None"}]
	}`, uniqueSlug)

	createReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/cases", strings.NewReader(createBody))
	require.NoError(t, err)
	createReq.Header.Set("Authorization", authHeader)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Idempotency-Key", newIdemKey())
	createResp, err := client.Do(createReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	var createdCase struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&createdCase))
	_ = createResp.Body.Close()
	createdCaseID = createdCase.ID
	require.NotEmpty(t, createdCaseID)

	// Confirm DRAFT case is NOT visible on public reader endpoints (404 uniform response).
	draftResp, err := client.Get(ts.URL + "/api/v1/cases/" + uniqueSlug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, draftResp.StatusCode)
	_ = draftResp.Body.Close()

	// 6. Upload real digital PDF fixture (contains "Meridian Clearing Partners" and "clearinghouse").
	fixturePath := filepath.Join("..", "..", "..", "processor-rust", "tests", "fixtures", "gov-press-release.pdf")
	pdfBytes, err := os.ReadFile(fixturePath)
	require.NoError(t, err, "expected Phase G fixture gov-press-release.pdf")

	var mpBuf bytes.Buffer
	mpWriter := multipart.NewWriter(&mpBuf)
	_ = mpWriter.WriteField("title", "DOJ Press Release 24-817 — Meridian Indictment")
	_ = mpWriter.WriteField("source_type", "gov_primary_record")
	_ = mpWriter.WriteField("license_status", "PUBLIC_DOMAIN")
	_ = mpWriter.WriteField("verification_tier", "1")
	_ = mpWriter.WriteField("reported_as_date", "2024-08-17")
	_ = mpWriter.WriteField("issuing_authority", "U.S. Department of Justice")
	_ = mpWriter.WriteField("source_citation", "DOJ OPA Press Release 24-817, SDNY Case 1:24-cr-00419")
	_ = mpWriter.WriteField("prohibited_content_cleared", "true")

	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="gov-press-release.pdf"`)
	partHeader.Set("Content-Type", "application/pdf")
	part, err := mpWriter.CreatePart(partHeader)
	require.NoError(t, err)
	_, err = part.Write(pdfBytes)
	require.NoError(t, err)
	require.NoError(t, mpWriter.Close())

	upReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/cases/"+createdCaseID+"/evidence", &mpBuf)
	require.NoError(t, err)
	upReq.Header.Set("Authorization", authHeader)
	upReq.Header.Set("Content-Type", mpWriter.FormDataContentType())
	upReq.Header.Set("Idempotency-Key", newIdemKey())
	upResp, err := client.Do(upReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, upResp.StatusCode)
	var upPayload struct {
		EvidenceID string `json:"evidence_id"`
	}
	require.NoError(t, json.NewDecoder(upResp.Body).Decode(&upPayload))
	_ = upResp.Body.Close()
	require.NotEmpty(t, upPayload.EvidenceID)

	// 7. Wait for real processor-rust worker to extract text and mark evidence COMPLETED in Postgres.
	var extractedText string
	require.Eventually(t, func() bool {
		var status string
		var txt *string
		qErr := st.Pool().QueryRow(
			ctx,
			`SELECT processing_status, extracted_text FROM evidence_items WHERE id = $1`,
			upPayload.EvidenceID,
		).Scan(&status, &txt)
		if qErr != nil || status != "COMPLETED" || txt == nil {
			return false
		}
		extractedText = *txt
		return true
	}, 20*time.Second, 150*time.Millisecond, "processor-rust must mark uploaded PDF COMPLETED")
	assert.Contains(t, strings.ToLower(extractedText), "clearinghouse")

	// Evidence on a DRAFT case must still return 404 publicly.
	draftEvResp, err := client.Get(ts.URL + "/api/v1/evidence/" + upPayload.EvidenceID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, draftEvResp.StatusCode)
	_ = draftEvResp.Body.Close()

	// 8. Record APPROVED legal review and publish case (synchronously indexes into Meilisearch).
	reviewBody := `{
		"decision": "APPROVED",
		"external_counsel_ref": "COUNSEL-2026-READER-001",
		"prohibited_checklist_passed": true,
		"genuine_gaps_verified": true,
		"license_audit_passed": true,
		"notes": "All primary DOJ filings verified."
	}`
	revReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/cases/"+createdCaseID+"/legal-review", strings.NewReader(reviewBody))
	require.NoError(t, err)
	revReq.Header.Set("Authorization", authHeader)
	revReq.Header.Set("Content-Type", "application/json")
	revReq.Header.Set("Idempotency-Key", newIdemKey())
	revResp, err := client.Do(revReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, revResp.StatusCode)
	_ = revResp.Body.Close()

	pubReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/cases/"+createdCaseID+"/publish", nil)
	require.NoError(t, err)
	pubReq.Header.Set("Authorization", authHeader)
	pubReq.Header.Set("Idempotency-Key", newIdemKey())
	pubResp, err := client.Do(pubReq)
	require.NoError(t, err)
	pubBodyBytes, _ := io.ReadAll(pubResp.Body)
	_ = pubResp.Body.Close()
	require.Equal(t, http.StatusOK, pubResp.StatusCode, "publish body: %s", string(pubBodyBytes))

	// 9. Verify anonymous public reader endpoints (no Authorization header).
	// 9a. GET /api/v1/cases
	listResp, err := client.Get(ts.URL + "/api/v1/cases?category=financial_crime")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, listResp.StatusCode)
	var listPayload struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listPayload))
	_ = listResp.Body.Close()
	foundInList := false
	for _, c := range listPayload.Data {
		if c.Slug == uniqueSlug {
			foundInList = true
			break
		}
	}
	assert.True(t, foundInList, "published case must appear in GET /api/v1/cases")

	// 9b. GET /api/v1/cases/{slug}
	detailResp, err := client.Get(ts.URL + "/api/v1/cases/" + uniqueSlug)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, detailResp.StatusCode)
	var detailPayload struct {
		Slug          string `json:"slug"`
		EvidenceCount int    `json:"evidence_count"`
		EvidenceItems []struct {
			ID               string `json:"id"`
			VerificationTier int    `json:"verification_tier"`
			LicenseStatus    string `json:"license_status"`
		} `json:"evidence_items"`
	}
	require.NoError(t, json.NewDecoder(detailResp.Body).Decode(&detailPayload))
	_ = detailResp.Body.Close()
	assert.Equal(t, uniqueSlug, detailPayload.Slug)
	assert.Equal(t, 1, detailPayload.EvidenceCount)
	require.Len(t, detailPayload.EvidenceItems, 1)
	assert.Equal(t, upPayload.EvidenceID, detailPayload.EvidenceItems[0].ID)

	// 9c. GET /api/v1/evidence/{id}
	evResp, err := client.Get(ts.URL + "/api/v1/evidence/" + upPayload.EvidenceID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, evResp.StatusCode)
	var evDetail struct {
		ID            string  `json:"id"`
		SignedFileURL *string `json:"signed_file_url"`
		PageCount     *int    `json:"page_count"`
		SHA256Hash    string  `json:"sha256_hash"`
	}
	require.NoError(t, json.NewDecoder(evResp.Body).Decode(&evDetail))
	_ = evResp.Body.Close()
	assert.Equal(t, upPayload.EvidenceID, evDetail.ID)
	require.NotNil(t, evDetail.SignedFileURL)
	assert.NotEmpty(t, *evDetail.SignedFileURL)
	require.NotNil(t, evDetail.PageCount)
	assert.GreaterOrEqual(t, *evDetail.PageCount, 2)
	assert.Len(t, evDetail.SHA256Hash, 64)

	// 9d. GET /api/v1/search?q=clearinghouse (term ONLY inside extracted PDF text, NOT in title/summary!)
	searchResp, err := client.Get(ts.URL + "/api/v1/search?q=clearinghouse")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, searchResp.StatusCode)
	var searchPayload struct {
		Data []struct {
			ID        string `json:"id"`
			Slug      string `json:"slug"`
			Highlight string `json:"highlight"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(searchResp.Body).Decode(&searchPayload))
	_ = searchResp.Body.Close()
	require.Len(t, searchPayload.Data, 1, "search for PDF-only term must return the published case")
	assert.Equal(t, uniqueSlug, searchPayload.Data[0].Slug)
	assert.Contains(t, strings.ToLower(searchPayload.Data[0].Highlight), "<mark>clearinghouse</mark>")

	// 10. Geoblock verification (CF-IPCountry: DE -> HTTP 451 static HTML, no case content; admin & /health unaffected).
	geoReq, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/cases/"+uniqueSlug, nil)
	require.NoError(t, err)
	geoReq.Header.Set("CF-IPCountry", "DE")
	geoResp, err := client.Do(geoReq)
	require.NoError(t, err)
	geoBytes, _ := io.ReadAll(geoResp.Body)
	_ = geoResp.Body.Close()
	assert.Equal(t, http.StatusUnavailableForLegalReasons, geoResp.StatusCode)
	assert.Contains(t, string(geoBytes), "Not available in your region")
	assert.NotContains(t, string(geoBytes), uniqueSlug)

	// /health and /api/v1/admin/* MUST bypass geoblock.
	healthDE, err := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	require.NoError(t, err)
	healthDE.Header.Set("CF-IPCountry", "DE")
	healthDEResp, err := client.Do(healthDE)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, healthDEResp.StatusCode)
	_ = healthDEResp.Body.Close()

	adminDE, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/cases/"+createdCaseID, nil)
	require.NoError(t, err)
	adminDE.Header.Set("CF-IPCountry", "DE")
	adminDE.Header.Set("Authorization", authHeader)
	adminDEResp, err := client.Do(adminDE)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, adminDEResp.StatusCode)
	_ = adminDEResp.Body.Close()

	// 11. Search rate limit (30/min per IP -> 31st request returns 429 + Retry-After).
	for i := 1; i <= 30; i++ {
		req, reqErr := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/search?q=clearinghouse", nil)
		require.NoError(t, reqErr)
		req.Header.Set("CF-Connecting-IP", searchTestIP)
		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "search request %d/30 should succeed", i)
	}
	rlReq, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/search?q=clearinghouse", nil)
	require.NoError(t, err)
	rlReq.Header.Set("CF-Connecting-IP", searchTestIP)
	rlResp, err := client.Do(rlReq)
	require.NoError(t, err)
	_ = rlResp.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, rlResp.StatusCode)
	assert.NotEmpty(t, rlResp.Header.Get("Retry-After"))

	// 12. POST /api/v1/corrections: valid -> 202 + DB row, honeypot -> silent 202 + NO DB row, 4th -> 429.
	countCorrections := func() int {
		var n int
		require.NoError(t, st.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM correction_requests WHERE case_id = $1`, createdCaseID).Scan(&n))
		return n
	}
	require.Equal(t, 0, countCorrections())

	// 12a. Valid correction #1 (with contact_email)
	corr1Body := fmt.Sprintf(`{
		"case_slug": %q,
		"description": "Exhibit citation year should reference 2024 rather than 2023.",
		"contact_email": "reader@example.org"
	}`, uniqueSlug)
	c1Req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/corrections", strings.NewReader(corr1Body))
	require.NoError(t, err)
	c1Req.Header.Set("Content-Type", "application/json")
	c1Req.Header.Set("CF-Connecting-IP", corrTestIP)
	c1Req.Header.Set("Idempotency-Key", newIdemKey())
	c1Resp, err := client.Do(c1Req)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, c1Resp.StatusCode)
	var c1Out struct {
		CorrectionID string `json:"correction_id"`
		Status       string `json:"status"`
	}
	require.NoError(t, json.NewDecoder(c1Resp.Body).Decode(&c1Out))
	_ = c1Resp.Body.Close()
	assert.NotEmpty(t, c1Out.CorrectionID)
	assert.Equal(t, "OPEN", c1Out.Status)
	assert.Equal(t, 1, countCorrections())

	// 12b. Honeypot submission #2 -> silent 202 and ZERO new DB rows!
	corr2Body := fmt.Sprintf(`{
		"case_slug": %q,
		"description": "Automated spam message that should never hit PostgreSQL.",
		"honeypot_field": "http://spam-bot.invalid"
	}`, uniqueSlug)
	c2Req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/corrections", strings.NewReader(corr2Body))
	require.NoError(t, err)
	c2Req.Header.Set("Content-Type", "application/json")
	c2Req.Header.Set("CF-Connecting-IP", corrTestIP)
	c2Req.Header.Set("Idempotency-Key", newIdemKey())
	c2Resp, err := client.Do(c2Req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, c2Resp.StatusCode)
	_ = c2Resp.Body.Close()
	assert.Equal(t, 1, countCorrections(), "honeypot submission MUST NOT write a DB row")

	// 12c. Valid correction #3 (optional contact_email omitted)
	corr3Body := fmt.Sprintf(`{
		"case_slug": %q,
		"description": "Page count reference on summary paragraph two appears off by one."
	}`, uniqueSlug)
	c3Req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/corrections", strings.NewReader(corr3Body))
	require.NoError(t, err)
	c3Req.Header.Set("Content-Type", "application/json")
	c3Req.Header.Set("CF-Connecting-IP", corrTestIP)
	c3Req.Header.Set("Idempotency-Key", newIdemKey())
	c3Resp, err := client.Do(c3Req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, c3Resp.StatusCode)
	_ = c3Resp.Body.Close()
	assert.Equal(t, 2, countCorrections())

	// 12d. 4th correction within 1 hour -> 429 Too Many Requests with Retry-After!
	c4Req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/corrections", strings.NewReader(corr3Body))
	require.NoError(t, err)
	c4Req.Header.Set("Content-Type", "application/json")
	c4Req.Header.Set("CF-Connecting-IP", corrTestIP)
	c4Req.Header.Set("Idempotency-Key", newIdemKey())
	c4Resp, err := client.Do(c4Req)
	require.NoError(t, err)
	_ = c4Resp.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, c4Resp.StatusCode)
	assert.NotEmpty(t, c4Resp.Header.Get("Retry-After"))

	// 13. Retract case -> removed from Meilisearch immediately and returns 404 on reader endpoints.
	retractReq, err := http.NewRequest(
		http.MethodPost,
		ts.URL+"/api/v1/admin/cases/"+createdCaseID+"/retract",
		strings.NewReader(`{"reason":"Post-publication verification audit"}`),
	)
	require.NoError(t, err)
	retractReq.Header.Set("Authorization", authHeader)
	retractReq.Header.Set("Content-Type", "application/json")
	retractReq.Header.Set("Idempotency-Key", newIdemKey())
	retractResp, err := client.Do(retractReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, retractResp.StatusCode)
	_ = retractResp.Body.Close()

	// Reader case detail & evidence detail now return 404.
	postRetractDetail, err := client.Get(ts.URL + "/api/v1/cases/" + uniqueSlug)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, postRetractDetail.StatusCode)
	_ = postRetractDetail.Body.Close()

	postRetractEv, err := client.Get(ts.URL + "/api/v1/evidence/" + upPayload.EvidenceID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, postRetractEv.StatusCode)
	_ = postRetractEv.Body.Close()

	// Search returns 0 hits for the retracted case.
	postRetractSearch, err := client.Get(ts.URL + "/api/v1/search?q=clearinghouse")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, postRetractSearch.StatusCode)
	var postSearchPayload struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(postRetractSearch.Body).Decode(&postSearchPayload))
	_ = postRetractSearch.Body.Close()
	assert.Empty(t, postSearchPayload.Data, "retracted case must be removed from Meilisearch immediately")
}

func startRustProcessor(
	t *testing.T,
	dbURL, redisURL, storageRoot, meiliURL, meiliAPIKey, meiliIndex string,
) func() {
	t.Helper()

	rustDir, err := filepath.Abs(filepath.Join("..", "..", "..", "processor-rust"))
	require.NoError(t, err)

	binPath := filepath.Join(rustDir, "target", "debug", "processor")
	if _, statErr := os.Stat(binPath); statErr != nil {
		buildCmd := exec.Command("cargo", "build", "--bin", "processor")
		buildCmd.Dir = rustDir
		out, buildErr := buildCmd.CombinedOutput()
		require.NoError(t, buildErr, "cargo build --bin processor failed: %s", string(out))
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	healthAddr := ln.Addr().String()
	_ = ln.Close()

	cmd := exec.Command(binPath)
	cmd.Dir = rustDir
	cmd.Env = append(os.Environ(),
		"DATABASE_URL="+dbURL,
		"REDIS_URL="+redisURL,
		"STORAGE_ROOT="+storageRoot,
		"MEILI_URL="+meiliURL,
		"MEILI_API_KEY="+meiliAPIKey,
		"MEILI_INDEX="+meiliIndex,
		"HEALTH_ADDR="+healthAddr,
		"WORKER_COUNT=1",
		"LOG_LEVEL=warn",
	)
	cmd.Stderr = &syncBuffer{}
	require.NoError(t, cmd.Start(), "start processor-rust worker binary")

	healthClient := &http.Client{Timeout: 500 * time.Millisecond}
	require.Eventually(t, func() bool {
		resp, reqErr := healthClient.Get("http://" + healthAddr + "/health")
		if reqErr != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 10*time.Second, 100*time.Millisecond, "processor-rust /health did not become ready")

	return func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			done := make(chan struct{})
			go func() {
				_ = cmd.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				_ = cmd.Process.Kill()
				<-done
			}
		}
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}
