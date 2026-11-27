package handlers

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubCaseWriter struct {
	byID   map[uuid.UUID]*models.Case
	bySlug map[string]*models.Case
}

func newStubCaseWriter() *stubCaseWriter {
	return &stubCaseWriter{
		byID:   make(map[uuid.UUID]*models.Case),
		bySlug: make(map[string]*models.Case),
	}
}

func (s *stubCaseWriter) Create(_ context.Context, in store.CreateCaseInput) (*models.Case, error) {
	if _, exists := s.bySlug[in.Slug]; exists {
		return nil, fmt.Errorf("slug %q already exists: %w", in.Slug, store.ErrConflict)
	}
	now := time.Now().UTC()
	c := &models.Case{
		ID:               uuid.New(),
		Slug:             in.Slug,
		Title:            in.Title,
		Category:         in.Category,
		Jurisdiction:     in.Jurisdiction,
		Status:           in.Status,
		OfficialBriefing: in.OfficialBriefing,
		GenuineGaps:      in.GenuineGaps,
		CreatedBy:        in.ActorID,
		UpdatedBy:        in.ActorID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.byID[c.ID] = c
	s.bySlug[c.Slug] = c
	return c, nil
}

func (s *stubCaseWriter) Update(_ context.Context, id uuid.UUID, in store.UpdateCaseInput) (*models.Case, error) {
	c, ok := s.byID[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	if c.Status != "DRAFT" && c.Status != "IN_REVIEW" {
		return nil, fmt.Errorf("Case must be retracted to DRAFT to edit.: %w", store.ErrConflict)
	}
	if in.Title != nil {
		c.Title = *in.Title
	}
	c.UpdatedBy = in.ActorID
	c.UpdatedAt = time.Now().UTC()
	return c, nil
}

func (s *stubCaseWriter) Retract(_ context.Context, id, actorID uuid.UUID, reason string) (*models.Case, error) {
	c, ok := s.byID[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	if c.Status != "PUBLISHED" {
		return nil, fmt.Errorf("not published: %w", store.ErrConflict)
	}
	now := time.Now().UTC()
	c.Status = "RETRACTED"
	c.RetractedAt = &now
	c.RetractionReason = &reason
	c.UpdatedBy = actorID
	return c, nil
}

func TestAdminCaseWriteHandler(t *testing.T) {
	t.Parallel()

	adminID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	jwtMgr, err := auth.NewJWTManager("case-write-unit-test-secret-at-least-32-chars")
	require.NoError(t, err)
	token, err := jwtMgr.Issue(adminID.String(), time.Hour)
	require.NoError(t, err)

	writer := newStubCaseWriter()
	pubID := uuid.New()
	writer.byID[pubID] = &models.Case{
		ID:       pubID,
		Slug:     "already-published",
		Title:    "Published Case",
		Category: "financial_crime",
		Status:   "PUBLISHED",
	}

	h := NewAdminCaseWriteHandler(writer)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Group(func(pr chi.Router) {
		pr.Use(middleware.RequireAdminJWT(jwtMgr))
		pr.Post("/api/v1/admin/cases", h.Create)
		pr.Put("/api/v1/admin/cases/{id}", h.Update)
		pr.Post("/api/v1/admin/cases/{id}/retract", h.Retract)
	})

	t.Run("create draft case succeeds and sets created_by/updated_by from JWT", func(t *testing.T) {
		body := `{"slug":"wire-fraud-2026","title":"Wire Fraud Dossier","summary":"DOJ indictment summary","category":"financial_crime"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
		var created models.Case
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
		assert.Equal(t, "wire-fraud-2026", created.Slug)
		assert.Equal(t, "DRAFT", created.Status)
		assert.Equal(t, adminID, created.CreatedBy)
		assert.Equal(t, adminID, created.UpdatedBy)
	})

	t.Run("duplicate slug returns 409 Conflict", func(t *testing.T) {
		body := `{"slug":"wire-fraud-2026","title":"Duplicate Slug Case","summary":"Duplicate","category":"financial_crime"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)
		var env middleware.ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		assert.Equal(t, "CONFLICT", env.Error.Code)
	})

	t.Run("create with status=PUBLISHED is rejected with 400", func(t *testing.T) {
		body := `{"slug":"bypass-publish","title":"Illegal Direct Publish","summary":"Nope","category":"financial_crime","status":"PUBLISHED"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("editing PUBLISHED case returns 409 with retract guidance", func(t *testing.T) {
		body := `{"title":"Attempted Edit On Published"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/cases/"+pubID.String(), strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)
		var env middleware.ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		assert.Equal(t, "Case must be retracted to DRAFT to edit.", env.Error.Message)
	})

	t.Run("retract PUBLISHED case transitions to RETRACTED with reason", func(t *testing.T) {
		body := `{"reason":"Legal notice received — emergency kill switch"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases/"+pubID.String()+"/retract", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var retracted models.Case
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &retracted))
		assert.Equal(t, "RETRACTED", retracted.Status)
		require.NotNil(t, retracted.RetractionReason)
		assert.Contains(t, *retracted.RetractionReason, "Legal notice")
	})
}
