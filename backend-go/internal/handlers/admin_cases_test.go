package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/auth"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/storage"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCaseReader struct {
	cases map[uuid.UUID]*models.Case
}

func (m mockCaseReader) GetByID(_ context.Context, id uuid.UUID) (*models.Case, error) {
	c, ok := m.cases[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return c, nil
}

type mockLegalReviewReader struct {
	reviews map[uuid.UUID]*models.LegalReview
}

func (m mockLegalReviewReader) GetByID(_ context.Context, id uuid.UUID) (*models.LegalReview, error) {
	lr, ok := m.reviews[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return lr, nil
}

type mockEvidenceLister struct {
	items map[uuid.UUID][]*models.EvidenceItem
}

func (m mockEvidenceLister) ListByCaseID(_ context.Context, caseID uuid.UUID) ([]*models.EvidenceItem, error) {
	return m.items[caseID], nil
}

func TestAdminCasesHandler_GetByID(t *testing.T) {
	t.Parallel()

	caseID := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	reviewID := uuid.MustParse("20000000-0000-0000-0000-000000000001")
	evidenceID := uuid.MustParse("40000000-0000-0000-0000-000000000001")
	adminID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	now := time.Now().UTC().Truncate(time.Second)

	caseStore := mockCaseReader{
		cases: map[uuid.UUID]*models.Case{
			caseID: {
				ID:                      caseID,
				Slug:                    "test-case-001",
				Title:                   "Test Case — Placeholder",
				Category:                "financial_crime",
				Jurisdiction:            "US-FEDERAL",
				Status:                  "DRAFT",
				OfficialBriefing:        "Briefing text",
				GenuineGaps:             json.RawMessage(`[]`),
				PublishedCorrectionsLog: json.RawMessage(`[]`),
				PublishedLegalReviewID:  &reviewID,
				CreatedBy:               adminID,
				UpdatedBy:               adminID,
				CreatedAt:               now,
				UpdatedAt:               now,
			},
		},
	}

	reviewStore := mockLegalReviewReader{
		reviews: map[uuid.UUID]*models.LegalReview{
			reviewID: {
				ID:                        reviewID,
				CaseID:                    caseID,
				ReviewerID:                adminID,
				ExternalCounselRef:        "DEV-SEED",
				Decision:                  "APPROVED",
				ProhibitedChecklistPassed: true,
				ReviewedAt:                now,
			},
		},
	}

	evidenceLister := mockEvidenceLister{
		items: map[uuid.UUID][]*models.EvidenceItem{
			caseID: {
				{
					ID:               evidenceID,
					CaseID:           caseID,
					Title:            "Federal Indictment",
					SourceType:       "gov_primary_record",
					VerificationTier: 1,
					LicenseStatus:    "PUBLIC_DOMAIN",
					MimeType:         "application/pdf",
					ByteSize:         1024,
					SHA256Hash:       strings.Repeat("a", 64),
					ObjectKey:        "cases/" + caseID.String() + "/evidence/" + evidenceID.String() + "/indictment.pdf",
					ProcessingStatus: "COMPLETED",
					RetrievedAt:      now,
				},
			},
		},
	}

	localStore, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	jwtMgr, err := auth.NewJWTManager("admin-cases-test-jwt-secret-minimum-32-chars")
	require.NoError(t, err)
	validToken, err := jwtMgr.Issue(adminID.String(), time.Hour)
	require.NoError(t, err)

	h := NewAdminCasesHandler(caseStore, reviewStore, evidenceLister, localStore)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Group(func(protected chi.Router) {
		protected.Use(middleware.RequireAdminJWT(jwtMgr))
		protected.Get("/api/v1/admin/cases/{id}", h.GetByID)
	})

	t.Run("200 with valid JWT returns case, review summary, and evidence items with signed URL", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/cases/"+caseID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp adminCaseDetailResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, caseID, resp.ID)
		assert.Equal(t, "test-case-001", resp.Slug)
		require.NotNil(t, resp.PublishedLegalReview)
		assert.Equal(t, reviewID, resp.PublishedLegalReview.ID)
		assert.Equal(t, "APPROVED", resp.PublishedLegalReview.Decision)
		assert.Equal(t, "DEV-SEED", resp.PublishedLegalReview.ExternalCounselRef)
		assert.True(t, resp.PublishedLegalReview.ProhibitedChecklistPassed)
		require.Len(t, resp.EvidenceItems, 1)
		assert.Equal(t, evidenceID, resp.EvidenceItems[0].ID)
		assert.True(t, strings.HasPrefix(resp.EvidenceItems[0].SignedURL, "file://"))
		assert.Contains(t, resp.EvidenceItems[0].SignedURL, "sig=")
	})

	t.Run("401 without JWT token", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/cases/"+caseID.String(), nil)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		var env middleware.ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		assert.Equal(t, "UNAUTHORIZED", env.Error.Code)
	})

	t.Run("404 for unknown case UUID", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/cases/"+uuid.NewString(), nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		var env middleware.ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		assert.Equal(t, "NOT_FOUND", env.Error.Code)
	})

	t.Run("400 for malformed UUID", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/cases/not-a-uuid", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var env middleware.ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		assert.Equal(t, "BAD_REQUEST", env.Error.Code)
	})
}
