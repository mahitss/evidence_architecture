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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mahitss/evidence_architecture/backend-go/internal/auth"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubPublishFlow struct {
	cases   map[uuid.UUID]*models.Case
	reviews map[uuid.UUID]*models.LegalReview
}

func (s *stubPublishFlow) Create(_ context.Context, in store.CreateLegalReviewInput) (*models.LegalReview, error) {
	c, ok := s.cases[in.CaseID]
	if !ok {
		return nil, store.ErrNotFound
	}
	lr := &models.LegalReview{
		ID:                        uuid.New(),
		CaseID:                    in.CaseID,
		ReviewerID:                in.ReviewerID,
		ExternalCounselRef:        in.ExternalCounselRef,
		Decision:                  in.Decision,
		ProhibitedChecklistPassed: in.ProhibitedChecklistPassed,
		GenuineGapsVerified:       in.GenuineGapsVerified,
		LicenseAuditPassed:        in.LicenseAuditPassed,
		Notes:                     in.Notes,
		ReviewedAt:                time.Now().UTC(),
	}
	s.reviews[lr.ID] = lr
	if lr.Decision == "APPROVED" {
		idCopy := lr.ID
		c.PublishedLegalReviewID = &idCopy
	}
	return lr, nil
}

func (s *stubPublishFlow) Publish(_ context.Context, id, actorID uuid.UUID) (*models.Case, error) {
	c, ok := s.cases[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	if c.PublishedLegalReviewID == nil {
		return nil, &pgconn.PgError{
			Code:    "P0001",
			Message: "Cannot publish case " + id.String() + ": published_legal_review_id is required (legal review gate)",
		}
	}
	lr, ok := s.reviews[*c.PublishedLegalReviewID]
	if !ok || lr.Decision != "APPROVED" || !lr.ProhibitedChecklistPassed {
		return nil, &pgconn.PgError{
			Code:    "P0001",
			Message: "Cannot publish case " + id.String() + ": no APPROVED legal_review_records row with prohibited_checklist_passed=TRUE",
		}
	}
	now := time.Now().UTC()
	c.Status = "PUBLISHED"
	c.PublishedAt = &now
	c.UpdatedBy = actorID
	return c, nil
}

func TestAdminPublishHandler_LegalReviewAndPublishGate(t *testing.T) {
	t.Parallel()

	adminID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	jwtMgr, err := auth.NewJWTManager("publish-handler-unit-test-secret-32-chars-min")
	require.NoError(t, err)
	token, err := jwtMgr.Issue(adminID.String(), time.Hour)
	require.NoError(t, err)

	unreviewedCaseID := uuid.New()
	approvedCaseID := uuid.New()
	rejectedCaseID := uuid.New()

	stub := &stubPublishFlow{
		cases: map[uuid.UUID]*models.Case{
			unreviewedCaseID: {ID: unreviewedCaseID, Slug: "unreviewed", Status: "DRAFT"},
			approvedCaseID:   {ID: approvedCaseID, Slug: "approved-flow", Status: "DRAFT"},
			rejectedCaseID:   {ID: rejectedCaseID, Slug: "rejected-flow", Status: "DRAFT"},
		},
		reviews: make(map[uuid.UUID]*models.LegalReview),
	}

	h := NewAdminPublishHandler(stub, stub)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Group(func(pr chi.Router) {
		pr.Use(middleware.RequireAdminJWT(jwtMgr))
		pr.Post("/api/v1/admin/cases/{id}/legal-review", h.RecordLegalReview)
		pr.Post("/api/v1/admin/cases/{id}/publish", h.Publish)
	})

	t.Run("publish without legal review returns 422 with exact PostgreSQL trigger message", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases/"+unreviewedCaseID.String()+"/publish", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var env middleware.ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		assert.Equal(t, "LEGAL_REVIEW_REQUIRED", env.Error.Code)
		assert.Contains(t, env.Error.Message, "published_legal_review_id is required")
	})

	t.Run("REJECTED legal review keeps case DRAFT and blocks publish with 422", func(t *testing.T) {
		revBody := `{"decision":"REJECTED","external_counsel_ref":"COUNSEL-REJ-1","prohibited_checklist_passed":true,"genuine_gaps_verified":false,"license_audit_passed":true,"notes":"Needs redaction"}`
		revReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases/"+rejectedCaseID.String()+"/legal-review", strings.NewReader(revBody))
		revReq.Header.Set("Authorization", "Bearer "+token)
		revRec := httptest.NewRecorder()
		r.ServeHTTP(revRec, revReq)
		require.Equal(t, http.StatusCreated, revRec.Code)
		assert.Equal(t, "DRAFT", stub.cases[rejectedCaseID].Status)

		pubReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases/"+rejectedCaseID.String()+"/publish", nil)
		pubReq.Header.Set("Authorization", "Bearer "+token)
		pubRec := httptest.NewRecorder()
		r.ServeHTTP(pubRec, pubReq)
		assert.Equal(t, http.StatusUnprocessableEntity, pubRec.Code)
	})

	t.Run("legacy defamation/privacy/copyright field names are rejected with 400", func(t *testing.T) {
		legacyBody := `{"decision":"APPROVED","external_counsel_ref":"COUNSEL-OLD","prohibited_checklist_passed":true,"defamation_check_passed":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases/"+approvedCaseID.String()+"/legal-review", strings.NewReader(legacyBody))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("APPROVED legal review links published_legal_review_id and publish succeeds", func(t *testing.T) {
		revBody := `{"decision":"APPROVED","external_counsel_ref":"COUNSEL-APP-1","prohibited_checklist_passed":true,"genuine_gaps_verified":true,"license_audit_passed":true,"notes":"Cleared"}`
		revReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases/"+approvedCaseID.String()+"/legal-review", strings.NewReader(revBody))
		revReq.Header.Set("Authorization", "Bearer "+token)
		revRec := httptest.NewRecorder()
		r.ServeHTTP(revRec, revReq)
		require.Equal(t, http.StatusCreated, revRec.Code)

		pubReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases/"+approvedCaseID.String()+"/publish", nil)
		pubReq.Header.Set("Authorization", "Bearer "+token)
		pubRec := httptest.NewRecorder()
		r.ServeHTTP(pubRec, pubReq)

		require.Equal(t, http.StatusOK, pubRec.Code)
		var published models.Case
		require.NoError(t, json.Unmarshal(pubRec.Body.Bytes(), &published))
		assert.Equal(t, "PUBLISHED", published.Status)
		assert.NotNil(t, published.PublishedLegalReviewID)
		assert.NotNil(t, published.PublishedAt)
	})
}
