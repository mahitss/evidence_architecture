package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/auth"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
	"github.com/mahitss/evidence_architecture/backend-go/internal/queue"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/storage"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type spyEvidenceWriter struct {
	createCalls int
	lastInput   store.CreateEvidenceInput
}

func (s *spyEvidenceWriter) Create(_ context.Context, in store.CreateEvidenceInput) (*models.EvidenceItem, error) {
	s.createCalls++
	s.lastInput = in
	return &models.EvidenceItem{
		ID:               in.ID,
		CaseID:           in.CaseID,
		Title:            in.Title,
		SourceType:       in.SourceType,
		VerificationTier: in.VerificationTier,
		LicenseStatus:    in.LicenseStatus,
		MimeType:         in.MimeType,
		ByteSize:         in.ByteSize,
		SHA256Hash:       in.SHA256Hash,
		ObjectKey:        in.ObjectKey,
		ProcessingStatus: "PENDING",
		RetrievedAt:      in.RetrievedAt,
	}, nil
}

func (s *spyEvidenceWriter) UpdateMetadata(_ context.Context, _ uuid.UUID, _ store.UpdateEvidenceInput) (*store.UpdateEvidenceResult, error) {
	return nil, store.ErrNotFound
}

type spyPublisher struct {
	jobs []queue.EvidenceProcessingJob
}

func (s *spyPublisher) PublishEvidenceJob(_ context.Context, job queue.EvidenceProcessingJob) error {
	s.jobs = append(s.jobs, job)
	return nil
}

func buildMultipartEvidenceRequest(t *testing.T, url, token string, fields map[string]string, fileContent []byte, contentType string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	for k, v := range fields {
		require.NoError(t, mw.WriteField(k, v))
	}

	if fileContent != nil {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="file"; filename="exhibit.pdf"`)
		h.Set("Content-Type", contentType)
		part, err := mw.CreatePart(h)
		require.NoError(t, err)
		_, err = part.Write(fileContent)
		require.NoError(t, err)
	}

	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func TestAdminEvidenceWriteHandler_ValidationAndUpload(t *testing.T) {
	t.Parallel()

	adminID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	caseID := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	jwtMgr, err := auth.NewJWTManager("evidence-write-unit-test-secret-32-chars-min")
	require.NoError(t, err)
	token, err := jwtMgr.Issue(adminID.String(), time.Hour)
	require.NoError(t, err)

	rejectionCases := []struct {
		name        string
		fields      map[string]string
		wantMessage string
	}{
		{
			name: "Q5 violation: third_party_exhibit + PUBLIC_DOMAIN returns 400 before DB hit",
			fields: map[string]string{
				"title":             "Third Party PACER Exhibit",
				"source_type":       "third_party_exhibit",
				"license_status":    "PUBLIC_DOMAIN",
				"verification_tier": "1",
				"reported_as_date":  "2026-01-15",
			},
			wantMessage: "Q5 violation: third-party exhibits cannot be PUBLIC_DOMAIN",
		},
		{
			name: "Recreated Tier 3 returns 400 before DB hit",
			fields: map[string]string{
				"title":                  "Recreated Tier 3 Diagram",
				"source_type":            "in_house_recreation",
				"license_status":         "RECREATED",
				"verification_tier":      "3",
				"recreation_source_ids":  `["src-1"]`,
				"recreation_author_note": "Recreated chart",
				"reported_as_date":       "2026-01-15",
			},
			wantMessage: "Recreations cannot claim Tier 3+.",
		},
		{
			name: "LICENSED without licensing_record_id returns 400 before DB hit",
			fields: map[string]string{
				"title":             "Licensed Exhibit Missing Record",
				"source_type":       "third_party_exhibit",
				"license_status":    "LICENSED",
				"verification_tier": "2",
				"reported_as_date":  "2026-01-15",
			},
			wantMessage: "licensing_record_id is required",
		},
		{
			name: "RECREATED without recreation_source_ids/author_note returns 400 before DB hit",
			fields: map[string]string{
				"title":             "Recreated Missing Attribution",
				"source_type":       "in_house_recreation",
				"license_status":    "RECREATED",
				"verification_tier": "2",
				"reported_as_date":  "2026-01-15",
			},
			wantMessage: "recreation_source_ids is required",
		},
	}

	for _, tc := range rejectionCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spyDB := &spyEvidenceWriter{}
			spyPub := &spyPublisher{}
			localStore, err := storage.NewLocalStorage(t.TempDir())
			require.NoError(t, err)

			h := NewAdminEvidenceWriteHandler(spyDB, localStore, spyPub, zerolog.Nop())
			r := chi.NewRouter()
			r.Use(middleware.RequestID)
			r.Group(func(pr chi.Router) {
				pr.Use(middleware.RequireAdminJWT(jwtMgr))
				pr.Post("/api/v1/admin/cases/{id}/evidence", h.Upload)
			})

			req := buildMultipartEvidenceRequest(
				t,
				fmt.Sprintf("/api/v1/admin/cases/%s/evidence", caseID),
				token,
				tc.fields,
				[]byte("%PDF-1.4\ntest content\n"),
				"application/pdf",
			)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, 0, spyDB.createCalls, "DB must NOT be hit on handler-level policy rejection")
			assert.Empty(t, spyPub.jobs, "Queue must NOT be published on policy rejection")

			var env middleware.ErrorEnvelopeResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
			assert.Contains(t, env.Error.Message, tc.wantMessage)
		})
	}

	t.Run("valid PUBLIC_DOMAIN gov_primary_record upload returns 202 PENDING and publishes job", func(t *testing.T) {
		t.Parallel()
		spyDB := &spyEvidenceWriter{}
		spyPub := &spyPublisher{}
		localStore, err := storage.NewLocalStorage(t.TempDir())
		require.NoError(t, err)

		h := NewAdminEvidenceWriteHandler(spyDB, localStore, spyPub, zerolog.Nop())
		r := chi.NewRouter()
		r.Use(middleware.RequestID)
		r.Group(func(pr chi.Router) {
			pr.Use(middleware.RequireAdminJWT(jwtMgr))
			pr.Post("/api/v1/admin/cases/{id}/evidence", h.Upload)
		})

		req := buildMultipartEvidenceRequest(
			t,
			fmt.Sprintf("/api/v1/admin/cases/%s/evidence", caseID),
			token,
			map[string]string{
				"title":             "DOJ Indictment PDF",
				"description":       "S.D.N.Y. 26-cr-0001 ECF No. 1",
				"source_type":       "gov_primary_record",
				"license_status":    "PUBLIC_DOMAIN",
				"verification_tier": "1",
				"reported_as_date":  "2026-01-15",
			},
			[]byte("%PDF-1.4\nvalid federal indictment bytes\n"),
			"application/pdf",
		)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusAccepted, rec.Code)
		assert.Equal(t, 1, spyDB.createCalls)
		assert.Len(t, spyPub.jobs, 1)

		var resp uploadEvidenceResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.NotEqual(t, uuid.Nil, resp.EvidenceID)
		assert.Equal(t, "PENDING", resp.ProcessingStatus)
		assert.True(t, resp.JobPublished)
		assert.Len(t, resp.SHA256Hash, 64)
		assert.Equal(t, resp.EvidenceID, spyPub.jobs[0].EvidenceID)
		assert.Equal(t, "application/pdf", spyPub.jobs[0].MimeType)
	})
}
