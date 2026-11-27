package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/storage"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
)

const signedEvidenceURLTTL = 5 * time.Minute

// CaseReader retrieves case dossiers by UUID.
type CaseReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Case, error)
}

// LegalReviewReader retrieves legal review sign-off records by UUID.
type LegalReviewReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.LegalReview, error)
}

// EvidenceLister retrieves evidence items belonging to a case.
type EvidenceLister interface {
	ListByCaseID(ctx context.Context, caseID uuid.UUID) ([]*models.EvidenceItem, error)
}

// AdminCasesHandler serves authenticated admin case read endpoints.
type AdminCasesHandler struct {
	cases    CaseReader
	reviews  LegalReviewReader
	evidence EvidenceLister
	storage  storage.Storage
}

// NewAdminCasesHandler constructs an AdminCasesHandler.
func NewAdminCasesHandler(
	cases CaseReader,
	reviews LegalReviewReader,
	evidence EvidenceLister,
	st storage.Storage,
) *AdminCasesHandler {
	return &AdminCasesHandler{
		cases:    cases,
		reviews:  reviews,
		evidence: evidence,
		storage:  st,
	}
}

type adminCaseDetailResponse struct {
	*models.Case
	PublishedLegalReview *models.LegalReviewSummary    `json:"published_legal_review"`
	EvidenceItems        []models.EvidenceMetadataItem `json:"evidence_items"`
}

// GetByID handles GET /api/v1/admin/cases/{id}, returning the case, its expanded
// minimal legal review summary, and its full metadata-only evidence_items list with signed URLs.
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

	evidenceList := make([]models.EvidenceMetadataItem, 0)
	if h.evidence != nil {
		rawItems, err := h.evidence.ListByCaseID(r.Context(), c.ID)
		if err != nil {
			middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "database unavailable")
			return
		}
		for _, item := range rawItems {
			signedURL := ""
			if h.storage != nil && item.ObjectKey != "" {
				u, err := h.storage.URL(r.Context(), item.ObjectKey, signedEvidenceURLTTL)
				if err == nil {
					signedURL = u
				}
			}
			evidenceList = append(evidenceList, item.ToMetadata(signedURL))
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(adminCaseDetailResponse{
		Case:                 c,
		PublishedLegalReview: reviewSummary,
		EvidenceItems:        evidenceList,
	})
}
