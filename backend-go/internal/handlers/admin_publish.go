package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
)

// LegalReviewWriter creates immutable legal_review_records rows.
type LegalReviewWriter interface {
	Create(ctx context.Context, in store.CreateLegalReviewInput) (*models.LegalReview, error)
}

// CasePublisher transitions a case to PUBLISHED via PostgreSQL trigger enforcement.
type CasePublisher interface {
	Publish(ctx context.Context, id, actorID uuid.UUID) (*models.Case, error)
}

// AdminPublishHandler serves POST /cases/{id}/legal-review and POST /cases/{id}/publish.
type AdminPublishHandler struct {
	reviews LegalReviewWriter
	cases   CasePublisher
}

// NewAdminPublishHandler constructs an AdminPublishHandler.
func NewAdminPublishHandler(reviews LegalReviewWriter, cases CasePublisher) *AdminPublishHandler {
	return &AdminPublishHandler{
		reviews: reviews,
		cases:   cases,
	}
}

type recordLegalReviewRequest struct {
	Decision                  string `json:"decision"`
	ExternalCounselRef        string `json:"external_counsel_ref"`
	ProhibitedChecklistPassed bool   `json:"prohibited_checklist_passed"`
	DefamationCheckPassed     *bool  `json:"defamation_check_passed"`
	PrivacyCheckPassed        *bool  `json:"privacy_check_passed"`
	CopyrightCheckPassed      *bool  `json:"copyright_check_passed"`
	GenuineGapsVerified       *bool  `json:"genuine_gaps_verified"`
	LicenseAuditPassed        *bool  `json:"license_audit_passed"`
	Notes                     string `json:"notes"`
}

// RecordLegalReview handles POST /api/v1/admin/cases/{id}/legal-review.
func (h *AdminPublishHandler) RecordLegalReview(w http.ResponseWriter, r *http.Request) {
	actorID, ok := adminActorIDFromRequest(w, r)
	if !ok {
		return
	}

	caseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid case id")
		return
	}

	var req recordLegalReviewRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, maxAdminJSONBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON request body")
		return
	}

	decision := strings.ToUpper(strings.TrimSpace(req.Decision))
	if decision != "APPROVED" && decision != "REJECTED" && decision != "CHANGES_REQUESTED" {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "decision must be APPROVED, REJECTED, or CHANGES_REQUESTED")
		return
	}

	counselRef := strings.TrimSpace(req.ExternalCounselRef)
	if counselRef == "" {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "external_counsel_ref is required")
		return
	}

	genuineGapsVerified := req.ProhibitedChecklistPassed
	if req.GenuineGapsVerified != nil {
		genuineGapsVerified = *req.GenuineGapsVerified
	} else if req.DefamationCheckPassed != nil && req.PrivacyCheckPassed != nil {
		genuineGapsVerified = *req.DefamationCheckPassed && *req.PrivacyCheckPassed
	}

	licenseAuditPassed := req.ProhibitedChecklistPassed
	if req.LicenseAuditPassed != nil {
		licenseAuditPassed = *req.LicenseAuditPassed
	} else if req.CopyrightCheckPassed != nil {
		licenseAuditPassed = *req.CopyrightCheckPassed
	}

	created, err := h.reviews.Create(r.Context(), store.CreateLegalReviewInput{
		CaseID:                    caseID,
		ReviewerID:                actorID,
		ExternalCounselRef:        counselRef,
		Decision:                  decision,
		ProhibitedChecklistPassed: req.ProhibitedChecklistPassed,
		GenuineGapsVerified:       genuineGapsVerified,
		LicenseAuditPassed:        licenseAuditPassed,
		Notes:                     req.Notes,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			middleware.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "case not found")
			return
		}
		if msg, ok := store.PGErrorMessage(err); ok && store.IsPGConstraintOrTriggerError(err) {
			middleware.WriteError(w, r, http.StatusUnprocessableEntity, "LEGAL_REVIEW_INVALID", msg)
			return
		}
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to record legal review")
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// Publish handles POST /api/v1/admin/cases/{id}/publish.
// Intentionally delegates precondition enforcement to the PostgreSQL trigger
// enforce_legal_review_before_publish (single source of truth) and surfaces any trigger
// or constraint rejection verbatim as HTTP 422.
func (h *AdminPublishHandler) Publish(w http.ResponseWriter, r *http.Request) {
	actorID, ok := adminActorIDFromRequest(w, r)
	if !ok {
		return
	}

	caseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid case id")
		return
	}

	published, err := h.cases.Publish(r.Context(), caseID, actorID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			middleware.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "case not found")
			return
		}
		if msg, ok := store.PGErrorMessage(err); ok && store.IsPGConstraintOrTriggerError(err) {
			middleware.WriteError(w, r, http.StatusUnprocessableEntity, "LEGAL_REVIEW_REQUIRED", msg)
			return
		}
		if errors.Is(err, store.ErrUnprocessable) {
			middleware.WriteError(w, r, http.StatusUnprocessableEntity, "LEGAL_REVIEW_REQUIRED", err.Error())
			return
		}
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to publish case")
		return
	}

	writeJSON(w, http.StatusOK, published)
}
