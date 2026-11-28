package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
	"github.com/mahitss/evidence_architecture/backend-go/internal/search"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
)

const maxAdminJSONBodyBytes = 1 << 20 // 1 MB

var validSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var validCaseCategories = map[string]struct{}{
	"financial_crime":          {},
	"historical_public_record": {},
}

// CaseWriter defines the persistence contract for admin case mutations.
type CaseWriter interface {
	Create(ctx context.Context, in store.CreateCaseInput) (*models.Case, error)
	Update(ctx context.Context, id uuid.UUID, in store.UpdateCaseInput) (*models.Case, error)
	Retract(ctx context.Context, id, actorID uuid.UUID, reason string) (*models.Case, error)
}

// AdminCaseWriteHandler serves POST /cases, PUT /cases/{id}, and POST /cases/{id}/retract.
type AdminCaseWriteHandler struct {
	cases   CaseWriter
	indexer search.Indexer
}

// NewAdminCaseWriteHandler creates an AdminCaseWriteHandler.
func NewAdminCaseWriteHandler(cases CaseWriter) *AdminCaseWriteHandler {
	return &AdminCaseWriteHandler{cases: cases}
}

// WithIndexer attaches a synchronous search indexer so retracted cases are immediately deleted from Meilisearch.
func (h *AdminCaseWriteHandler) WithIndexer(indexer search.Indexer) *AdminCaseWriteHandler {
	h.indexer = indexer
	return h
}

type createCaseRequest struct {
	Slug             string          `json:"slug"`
	Title            string          `json:"title"`
	Summary          string          `json:"summary"`
	OfficialBriefing string          `json:"official_briefing"`
	Category         string          `json:"category"`
	Jurisdiction     string          `json:"jurisdiction"`
	EUOrigin         bool            `json:"eu_origin"`
	GDPRVerified     bool            `json:"gdpr_verified"`
	Status           string          `json:"status"`
	GenuineGaps      json.RawMessage `json:"genuine_gaps"`
}

// Create handles POST /api/v1/admin/cases.
func (h *AdminCaseWriteHandler) Create(w http.ResponseWriter, r *http.Request) {
	actorID, ok := adminActorIDFromRequest(w, r)
	if !ok {
		return
	}

	var req createCaseRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, maxAdminJSONBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON request body")
		return
	}

	slug := strings.TrimSpace(req.Slug)
	if !validSlugPattern.MatchString(slug) {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "slug must match ^[a-z0-9-]+$")
		return
	}

	title := strings.TrimSpace(req.Title)
	if len(title) < 3 || len(title) > 200 {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "title must be between 3 and 200 characters")
		return
	}

	category := strings.TrimSpace(req.Category)
	if _, ok := validCaseCategories[category]; !ok {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "category must be financial_crime or historical_public_record")
		return
	}

	status := strings.ToUpper(strings.TrimSpace(req.Status))
	if status == "" {
		status = "DRAFT"
	}
	if status == "PUBLISHED" {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "cannot create a case directly in PUBLISHED status; use POST /publish after legal review")
		return
	}
	if status != "DRAFT" && status != "IN_REVIEW" {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "initial case status must be DRAFT or IN_REVIEW")
		return
	}

	briefing := req.OfficialBriefing
	if strings.TrimSpace(briefing) == "" {
		briefing = req.Summary
	}

	jurisdiction := strings.TrimSpace(req.Jurisdiction)
	if jurisdiction == "" {
		jurisdiction = "US"
	}

	gaps := req.GenuineGaps
	if len(gaps) == 0 {
		gaps = json.RawMessage(`[]`)
	} else if !json.Valid(gaps) {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "genuine_gaps must be valid JSON")
		return
	}

	created, err := h.cases.Create(r.Context(), store.CreateCaseInput{
		Slug:             slug,
		Title:            title,
		Category:         category,
		Jurisdiction:     jurisdiction,
		EUOrigin:         req.EUOrigin,
		GDPRVerified:     req.GDPRVerified,
		Status:           status,
		OfficialBriefing: briefing,
		GenuineGaps:      gaps,
		ActorID:          actorID,
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			middleware.WriteError(w, r, http.StatusConflict, "CONFLICT", "case slug already exists")
			return
		}
		if msg, ok := store.PGErrorMessage(err); ok && store.IsPGConstraintOrTriggerError(err) {
			middleware.WriteError(w, r, http.StatusUnprocessableEntity, "UNPROCESSABLE_ENTITY", msg)
			return
		}
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to create case")
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

type updateCaseRequest struct {
	Slug             *string         `json:"slug"`
	Title            *string         `json:"title"`
	Summary          *string         `json:"summary"`
	OfficialBriefing *string         `json:"official_briefing"`
	Category         *string         `json:"category"`
	Jurisdiction     *string         `json:"jurisdiction"`
	EUOrigin         *bool           `json:"eu_origin"`
	GDPRVerified     *bool           `json:"gdpr_verified"`
	GenuineGaps      json.RawMessage `json:"genuine_gaps"`
}

// Update handles PUT /api/v1/admin/cases/{id}.
func (h *AdminCaseWriteHandler) Update(w http.ResponseWriter, r *http.Request) {
	actorID, ok := adminActorIDFromRequest(w, r)
	if !ok {
		return
	}

	caseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid case id")
		return
	}

	var req updateCaseRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, maxAdminJSONBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON request body")
		return
	}

	var in store.UpdateCaseInput
	in.ActorID = actorID

	if req.Slug != nil {
		s := strings.TrimSpace(*req.Slug)
		if !validSlugPattern.MatchString(s) {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "slug must match ^[a-z0-9-]+$")
			return
		}
		in.Slug = &s
	}
	if req.Title != nil {
		t := strings.TrimSpace(*req.Title)
		if len(t) < 3 || len(t) > 200 {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "title must be between 3 and 200 characters")
			return
		}
		in.Title = &t
	}
	if req.Category != nil {
		cat := strings.TrimSpace(*req.Category)
		if _, ok := validCaseCategories[cat]; !ok {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "category must be financial_crime or historical_public_record")
			return
		}
		in.Category = &cat
	}
	if req.Jurisdiction != nil {
		j := strings.TrimSpace(*req.Jurisdiction)
		in.Jurisdiction = &j
	}
	in.EUOrigin = req.EUOrigin
	in.GDPRVerified = req.GDPRVerified

	if req.OfficialBriefing != nil {
		in.OfficialBriefing = req.OfficialBriefing
	} else if req.Summary != nil {
		in.OfficialBriefing = req.Summary
	}

	if len(req.GenuineGaps) > 0 {
		if !json.Valid(req.GenuineGaps) {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "genuine_gaps must be valid JSON")
			return
		}
		in.GenuineGaps = req.GenuineGaps
	}

	updated, err := h.cases.Update(r.Context(), caseID, in)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			middleware.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "case not found")
			return
		}
		if errors.Is(err, store.ErrConflict) {
			msg := "Case must be retracted to DRAFT to edit."
			if strings.Contains(err.Error(), "already exists") {
				msg = "case slug already exists"
			}
			middleware.WriteError(w, r, http.StatusConflict, "CONFLICT", msg)
			return
		}
		if msg, ok := store.PGErrorMessage(err); ok && store.IsPGConstraintOrTriggerError(err) {
			middleware.WriteError(w, r, http.StatusUnprocessableEntity, "UNPROCESSABLE_ENTITY", msg)
			return
		}
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to update case")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

type retractCaseRequest struct {
	Reason     string `json:"reason"`
	ReasonCode string `json:"reason_code"`
}

// Retract handles POST /api/v1/admin/cases/{id}/retract.
func (h *AdminCaseWriteHandler) Retract(w http.ResponseWriter, r *http.Request) {
	actorID, ok := adminActorIDFromRequest(w, r)
	if !ok {
		return
	}

	caseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid case id")
		return
	}

	var req retractCaseRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, maxAdminJSONBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON request body")
		return
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = strings.TrimSpace(req.ReasonCode)
	}
	if reason == "" {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "reason is required to retract a published case")
		return
	}

	retracted, err := h.cases.Retract(r.Context(), caseID, actorID, reason)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			middleware.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "case not found")
			return
		}
		if errors.Is(err, store.ErrConflict) {
			middleware.WriteError(w, r, http.StatusConflict, "CONFLICT", "only PUBLISHED cases can be retracted")
			return
		}
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to retract case")
		return
	}

	if h.indexer != nil {
		if err := h.indexer.DeleteCase(r.Context(), retracted.ID); err != nil {
			middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "case retracted in database but failed to remove from search index")
			return
		}
	}

	writeJSON(w, http.StatusOK, retracted)
}

func adminActorIDFromRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok || claims.Subject == "" {
		middleware.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated admin claims")
		return uuid.Nil, false
	}
	actorID, err := uuid.Parse(claims.Subject)
	if err != nil {
		middleware.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "invalid admin subject claim")
		return uuid.Nil, false
	}
	return actorID, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
