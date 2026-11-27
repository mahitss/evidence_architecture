package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
)

// CaseReader retrieves case dossiers by UUID.
type CaseReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Case, error)
}

// LegalReviewReader retrieves legal review sign-off records by UUID.
type LegalReviewReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.LegalReview, error)
}

// AdminCasesHandler serves authenticated admin case read endpoints.
type AdminCasesHandler struct {
	cases   CaseReader
	reviews LegalReviewReader
}

// NewAdminCasesHandler constructs an AdminCasesHandler.
func NewAdminCasesHandler(cases CaseReader, reviews LegalReviewReader) *AdminCasesHandler {
	return &AdminCasesHandler{
		cases:   cases,
		reviews: reviews,
	}
}

type adminCaseDetailResponse struct {
	*models.Case
	PublishedLegalReview *models.LegalReviewSummary `json:"published_legal_review"`
}

// GetByID handles GET /api/v1/admin/cases/{id}.
func (h *AdminCasesHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "id")
	caseID, err := uuid.Parse(rawID)
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid case id")
		return
	}

	c, err := h.cases.GetByID(r.Context(), caseID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			middleware.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "case not found")
			return
		}
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "database unavailable")
		return
	}

	var reviewSummary *models.LegalReviewSummary
	if c.PublishedLegalReviewID != nil && h.reviews != nil {
		lr, err := h.reviews.GetByID(r.Context(), *c.PublishedLegalReviewID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "database unavailable")
			return
		}
		if lr != nil {
			reviewSummary = lr.Summary()
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(adminCaseDetailResponse{
		Case:                 c,
		PublishedLegalReview: reviewSummary,
	})
}
