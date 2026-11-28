package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
	"github.com/mahitss/evidence_architecture/backend-go/internal/search"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/storage"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
)

const (
	maxCorrectionBodyBytes = 16 << 10 // 16 KB per SECURITY_ARCHITECTURE.md T4
	readerSignedURLTTL     = 5 * time.Minute
)

var validCorrectionRequestTypes = map[string]struct{}{
	"factual_correction":  {},
	"defamation_takedown": {},
	"copyright_takedown":  {},
	"privacy_grievance":   {},
}

var basicEmailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// ReaderCaseStore defines read-only published case and timeline queries.
type ReaderCaseStore interface {
	ListPublished(ctx context.Context, category string, cursor string, limit int) ([]store.PublishedCaseSummary, *string, bool, error)
	GetPublishedBySlug(ctx context.Context, slug string) (*models.Case, error)
	ListTimelineByCaseID(ctx context.Context, caseID uuid.UUID) ([]models.TimelineEvent, error)
}

// ReaderEvidenceStore defines read-only published evidence queries.
type ReaderEvidenceStore interface {
	ListByCaseID(ctx context.Context, caseID uuid.UUID) ([]*models.EvidenceItem, error)
	GetPublishedByID(ctx context.Context, id uuid.UUID) (*models.EvidenceItem, error)
}

// ReaderCorrectionStore defines persistence for public correction requests.
type ReaderCorrectionStore interface {
	Create(ctx context.Context, in store.CreateCorrectionInput) (*models.CorrectionRequest, error)
}

// ReaderHandler serves anonymous public reader endpoints (no authentication, EU/EEA geoblocked).
type ReaderHandler struct {
	cases       ReaderCaseStore
	evidence    ReaderEvidenceStore
	corrections ReaderCorrectionStore
	storage     storage.Storage
	search      search.Indexer
}

// NewReaderHandler constructs a ReaderHandler.
func NewReaderHandler(
	cases ReaderCaseStore,
	evidence ReaderEvidenceStore,
	corrections ReaderCorrectionStore,
	objStorage storage.Storage,
	searchIndexer search.Indexer,
) *ReaderHandler {
	return &ReaderHandler{
		cases:       cases,
		evidence:    evidence,
		corrections: corrections,
		storage:     objStorage,
		search:      searchIndexer,
	}
}

type caseListResponse struct {
	Data       []store.PublishedCaseSummary `json:"data"`
	NextCursor *string                      `json:"next_cursor"`
	HasMore    bool                         `json:"has_more"`
}

// ListCases handles GET /api/v1/cases.
// Returns cursor-paginated PUBLISHED cases only. Rejects non-PUBLISHED status params (T1).
func (h *ReaderHandler) ListCases(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	if rawStatus := strings.TrimSpace(q.Get("status")); rawStatus != "" && rawStatus != "PUBLISHED" {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "only PUBLISHED status is accessible on public endpoints")
		return
	}

	category := strings.TrimSpace(q.Get("category"))
	if category != "" {
		if _, ok := validCaseCategories[category]; !ok {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "category must be financial_crime or historical_public_record")
			return
		}
	}

	limit, err := parseLimitParam(q.Get("limit"), 20, 50)
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	cursor := strings.TrimSpace(q.Get("cursor"))
	items, nextCursor, hasMore, err := h.cases.ListPublished(r.Context(), category, cursor, limit)
	if err != nil {
		if errors.Is(err, store.ErrInvalidCursor) {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid pagination cursor")
			return
		}
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to list published cases")
		return
	}

	writeJSON(w, http.StatusOK, caseListResponse{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	})
}

type readerEvidenceSummary struct {
	ID               uuid.UUID `json:"id"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Type             string    `json:"type"`
	SourceType       string    `json:"source_type"`
	VerificationTier int16     `json:"verification_tier"`
	LicenseStatus    string    `json:"license_status"`
	SourceCitation   string    `json:"source_citation"`
	SHA256Hash       string    `json:"sha256_hash"`
	IsRedacted       bool      `json:"is_redacted"`
	WarningLevel     string    `json:"warning_level"`
}

type readerCaseDetailResponse struct {
	ID               uuid.UUID               `json:"id"`
	Slug             string                  `json:"slug"`
	Title            string                  `json:"title"`
	Summary          string                  `json:"summary"`
	OfficialBriefing string                  `json:"official_briefing"`
	Category         string                  `json:"category"`
	Jurisdiction     string                  `json:"jurisdiction"`
	PublishedAt      *time.Time              `json:"published_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
	EvidenceCount    int                     `json:"evidence_count"`
	GenuineGaps      json.RawMessage         `json:"genuine_gaps"`
	GenuineGapsNote  string                  `json:"genuine_gaps_note"`
	EvidenceItems    []readerEvidenceSummary `json:"evidence_items"`
	Timeline         []models.TimelineEvent  `json:"timeline"`
	TimelineEvents   []models.TimelineEvent  `json:"timeline_events"`
	CorrectionsLog   json.RawMessage         `json:"corrections_log"`
}

// GetCaseBySlug handles GET /api/v1/cases/{slug}.
// Returns uniform 404 if the case does not exist OR status != PUBLISHED (T1).
func (h *ReaderHandler) GetCaseBySlug(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(chi.URLParam(r, "slug"))
	if !validSlugPattern.MatchString(slug) || len(slug) < 3 || len(slug) > 120 {
		middleware.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "case not found")
		return
	}

	caseRec, err := h.cases.GetPublishedBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			middleware.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "case not found")
			return
		}
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to load case")
		return
	}

	evItems, err := h.evidence.ListByCaseID(r.Context(), caseRec.ID)
	if err != nil {
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to load case evidence")
		return
	}

	timeline, err := h.cases.ListTimelineByCaseID(r.Context(), caseRec.ID)
	if err != nil {
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to load case timeline")
		return
	}
	if timeline == nil {
		timeline = make([]models.TimelineEvent, 0)
	}

	summaries := make([]readerEvidenceSummary, 0, len(evItems))
	for _, ev := range evItems {
		summaries = append(summaries, readerEvidenceSummary{
			ID:               ev.ID,
			Title:            ev.Title,
			Description:      ev.SourceCitation,
			Type:             ev.SourceType,
			SourceType:       ev.SourceType,
			VerificationTier: ev.VerificationTier,
			LicenseStatus:    ev.LicenseStatus,
			SourceCitation:   ev.SourceCitation,
			SHA256Hash:       ev.SHA256Hash,
			IsRedacted:       ev.IsRedacted,
			WarningLevel:     computeEvidenceWarningLevel(ev.VerificationTier, ev.LicenseStatus),
		})
	}

	gaps := caseRec.GenuineGaps
	if len(gaps) == 0 {
		gaps = json.RawMessage(`[]`)
	}
	correctionsLog := caseRec.PublishedCorrectionsLog
	if len(correctionsLog) == 0 {
		correctionsLog = json.RawMessage(`[]`)
	}

	writeJSON(w, http.StatusOK, readerCaseDetailResponse{
		ID:               caseRec.ID,
		Slug:             caseRec.Slug,
		Title:            caseRec.Title,
		Summary:          caseRec.OfficialBriefing,
		OfficialBriefing: caseRec.OfficialBriefing,
		Category:         caseRec.Category,
		Jurisdiction:     caseRec.Jurisdiction,
		PublishedAt:      caseRec.PublishedAt,
		UpdatedAt:        caseRec.UpdatedAt,
		EvidenceCount:    len(summaries),
		GenuineGaps:      gaps,
		GenuineGapsNote:  formatGenuineGapsNote(gaps),
		EvidenceItems:    summaries,
		Timeline:         timeline,
		TimelineEvents:   timeline,
		CorrectionsLog:   correctionsLog,
	})
}

type readerEvidenceDetailResponse struct {
	ID                   uuid.UUID  `json:"id"`
	CaseID               uuid.UUID  `json:"case_id"`
	Title                string     `json:"title"`
	Description          string     `json:"description"`
	Type                 string     `json:"type"`
	SourceType           string     `json:"source_type"`
	VerificationTier     int16      `json:"verification_tier"`
	LicenseStatus        string     `json:"license_status"`
	LicenseBasisNote     string     `json:"license_basis_note"`
	SourceCitation       string     `json:"source_citation"`
	IssuingAuthority     string     `json:"issuing_authority"`
	SourceURL            *string    `json:"source_url"`
	StorageURL           *string    `json:"storage_url"`
	SignedFileURL        *string    `json:"signed_file_url"`
	SignedURLExpiresAt   *time.Time `json:"signed_url_expires_at"`
	RecreationAuthorNote *string    `json:"recreation_author_note"`
	MimeType             string     `json:"mime_type"`
	ByteSize             int64      `json:"byte_size"`
	PageCount            *int       `json:"page_count"`
	OCRUsed              bool       `json:"ocr_used"`
	SHA256Hash           string     `json:"sha256_hash"`
	IsRedacted           bool       `json:"is_redacted"`
	RedactionReason      *string    `json:"redaction_reason"`
	WarningLevel         string     `json:"warning_level"`
	RetrievedAt          time.Time  `json:"retrieved_at"`
}

// GetEvidenceByID handles GET /api/v1/evidence/{id}.
// Returns 404 if the evidence item does not exist OR its parent case is not PUBLISHED.
func (h *ReaderHandler) GetEvidenceByID(w http.ResponseWriter, r *http.Request) {
	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "evidence not found")
		return
	}

	ev, err := h.evidence.GetPublishedByID(r.Context(), evidenceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			middleware.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "evidence not found")
			return
		}
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to load evidence")
		return
	}

	var signedURL *string
	var expiresAt *time.Time
	if !ev.IsRedacted && h.storage != nil {
		urlStr, signErr := h.storage.URL(r.Context(), ev.ObjectKey, readerSignedURLTTL)
		if signErr != nil {
			middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to sign evidence URL")
			return
		}
		signedURL = &urlStr
		exp := time.Now().UTC().Add(readerSignedURLTTL)
		expiresAt = &exp
	}

	writeJSON(w, http.StatusOK, readerEvidenceDetailResponse{
		ID:                   ev.ID,
		CaseID:               ev.CaseID,
		Title:                ev.Title,
		Description:          ev.SourceCitation,
		Type:                 ev.SourceType,
		SourceType:           ev.SourceType,
		VerificationTier:     ev.VerificationTier,
		LicenseStatus:        ev.LicenseStatus,
		LicenseBasisNote:     ev.LicenseBasisNote,
		SourceCitation:       ev.SourceCitation,
		IssuingAuthority:     ev.IssuingAuthority,
		SourceURL:            ev.SourceURL,
		StorageURL:           signedURL,
		SignedFileURL:        signedURL,
		SignedURLExpiresAt:   expiresAt,
		RecreationAuthorNote: ev.RecreationAuthorNote,
		MimeType:             ev.MimeType,
		ByteSize:             ev.ByteSize,
		PageCount:            ev.PageCount,
		OCRUsed:              ev.OCRUsed,
		SHA256Hash:           ev.SHA256Hash,
		IsRedacted:           ev.IsRedacted,
		RedactionReason:      ev.RedactionReason,
		WarningLevel:         computeEvidenceWarningLevel(ev.VerificationTier, ev.LicenseStatus),
		RetrievedAt:          ev.RetrievedAt,
	})
}

type searchListResponse struct {
	Data       []search.SearchHit `json:"data"`
	NextCursor *string            `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
}

// Search handles GET /api/v1/search.
// Enforces query length (1-200 chars), strict category allowlist, and unconditional PUBLISHED-only filter (T5).
func (h *ReaderHandler) Search(w http.ResponseWriter, r *http.Request) {
	if h.search == nil {
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "search service unavailable")
		return
	}

	q := r.URL.Query()
	queryText := strings.TrimSpace(q.Get("q"))
	if len(queryText) == 0 || len(queryText) > search.MaxQueryLength {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "q is required and must be between 1 and 200 characters")
		return
	}

	category := strings.TrimSpace(q.Get("category"))
	if category != "" {
		if _, ok := validCaseCategories[category]; !ok {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "category must be financial_crime or historical_public_record")
			return
		}
	}

	limit, err := parseLimitParam(q.Get("limit"), 20, 50)
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	offset, err := decodeOffsetCursor(strings.TrimSpace(q.Get("cursor")))
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid search cursor")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	res, err := h.search.Search(ctx, queryText, search.SearchFilters{
		Category:      category,
		OnlyPublished: true,
	}, limit, offset)
	if err != nil {
		if errors.Is(err, search.ErrInvalidQuery) || errors.Is(err, search.ErrInvalidCategory) {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "search service temporarily unavailable")
		return
	}

	var nextCursor *string
	if res.HasMore {
		enc := encodeOffsetCursor(offset + limit)
		nextCursor = &enc
	}

	hits := res.Hits
	if hits == nil {
		hits = make([]search.SearchHit, 0)
	}

	writeJSON(w, http.StatusOK, searchListResponse{
		Data:       hits,
		NextCursor: nextCursor,
		HasMore:    res.HasMore,
	})
}

type submitCorrectionRequest struct {
	CaseSlug              string  `json:"case_slug"`
	EvidenceID            *string `json:"evidence_id"`
	RequestType           string  `json:"request_type"`
	ContactEmail          string  `json:"contact_email"`
	RequesterContactEmail string  `json:"requester_contact_email"`
	Description           string  `json:"description"`
	SupportingCitation    *string `json:"supporting_citation"`
	HoneypotField         string  `json:"honeypot_field"`
	Website               string  `json:"website"`
}

type submitCorrectionResponse struct {
	CorrectionID uuid.UUID `json:"correction_id"`
	TicketID     string    `json:"ticket_id"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// SubmitCorrection handles POST /api/v1/corrections.
// Anonymous, geoblocked, rate-limited (3/hr per IP), with silent bot honeypot rejection (T4).
func (h *ReaderHandler) SubmitCorrection(w http.ResponseWriter, r *http.Request) {
	var req submitCorrectionRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, maxCorrectionBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON request body")
		return
	}

	// Honeypot check (T4): non-empty honeypot_field or website silently returns 202 without writing to PostgreSQL.
	if strings.TrimSpace(req.HoneypotField) != "" || strings.TrimSpace(req.Website) != "" {
		writeJSON(w, http.StatusAccepted, submitCorrectionResponse{
			CorrectionID: uuid.New(),
			TicketID:     "COR-ACCEPTED",
			Status:       "OPEN",
			CreatedAt:    time.Now().UTC(),
		})
		return
	}

	description := strings.TrimSpace(req.Description)
	if len(description) < 10 || len(description) > 5000 {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "description must be between 10 and 5000 characters")
		return
	}

	reqType := strings.TrimSpace(req.RequestType)
	if reqType == "" {
		reqType = "factual_correction"
	}
	if _, ok := validCorrectionRequestTypes[reqType]; !ok {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid request_type")
		return
	}

	email := strings.TrimSpace(req.ContactEmail)
	if email == "" {
		email = strings.TrimSpace(req.RequesterContactEmail)
	}
	if email != "" && !basicEmailRegex.MatchString(email) {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "contact_email must be a valid email address")
		return
	}

	var evidenceID *uuid.UUID
	if req.EvidenceID != nil && strings.TrimSpace(*req.EvidenceID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.EvidenceID))
		if err != nil {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "evidence_id must be a valid UUID")
			return
		}
		evidenceID = &parsed
	}

	if req.SupportingCitation != nil && len(strings.TrimSpace(*req.SupportingCitation)) > 1000 {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "supporting_citation must be <= 1000 characters")
		return
	}

	created, err := h.corrections.Create(r.Context(), store.CreateCorrectionInput{
		CaseSlug:              strings.TrimSpace(req.CaseSlug),
		EvidenceID:            evidenceID,
		RequestType:           reqType,
		RequesterContactEmail: email,
		Description:           description,
		SupportingCitation:    req.SupportingCitation,
	})
	if err != nil {
		if msg, ok := store.PGErrorMessage(err); ok && store.IsPGConstraintOrTriggerError(err) {
			middleware.WriteError(w, r, http.StatusUnprocessableEntity, "UNPROCESSABLE_ENTITY", msg)
			return
		}
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to record correction request")
		return
	}

	writeJSON(w, http.StatusAccepted, submitCorrectionResponse{
		CorrectionID: created.ID,
		TicketID:     created.TicketID,
		Status:       created.Status,
		CreatedAt:    created.CreatedAt,
	})
}

func parseLimitParam(raw string, def, maxVal int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > maxVal {
		return 0, fmt.Errorf("limit must be an integer between 1 and %d", maxVal)
	}
	return n, nil
}

func encodeOffsetCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("off:%d", offset)))
}

func decodeOffsetCursor(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, errors.New("malformed cursor")
	}
	s := string(b)
	if !strings.HasPrefix(s, "off:") {
		return 0, errors.New("malformed cursor")
	}
	n, err := strconv.Atoi(strings.TrimPrefix(s, "off:"))
	if err != nil || n < 0 {
		return 0, errors.New("malformed cursor")
	}
	return n, nil
}

func computeEvidenceWarningLevel(tier int16, licenseStatus string) string {
	switch {
	case tier == 4:
		return "HIGH_WARNING"
	case tier == 3:
		return "WARNING"
	case licenseStatus == "RECREATED":
		return "DOUBLE_WARNING"
	default:
		return "NONE"
	}
}

func formatGenuineGapsNote(raw json.RawMessage) string {
	var entries []map[string]any
	if err := json.Unmarshal(raw, &entries); err != nil || len(entries) == 0 {
		return ""
	}
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		if claim, ok := entry["claim"].(string); ok && strings.TrimSpace(claim) != "" {
			if why, ok := entry["why_unverified"].(string); ok && strings.TrimSpace(why) != "" {
				parts = append(parts, fmt.Sprintf("%s (%s)", strings.TrimSpace(claim), strings.TrimSpace(why)))
			} else {
				parts = append(parts, strings.TrimSpace(claim))
			}
			continue
		}
		if question, ok := entry["question"].(string); ok && strings.TrimSpace(question) != "" {
			parts = append(parts, strings.TrimSpace(question))
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "; ")
	}
	return strings.TrimSpace(string(raw))
}
