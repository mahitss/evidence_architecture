package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
	"github.com/mahitss/evidence_architecture/backend-go/internal/queue"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/storage"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
	"github.com/rs/zerolog"
)

const (
	maxEvidenceUploadBytes int64 = 26214400 // 25 MB DB chk_byte_size cap
	multipartMemoryBytes   int64 = 4 << 20  // 4 MB in-memory buffer before temp file spill
)

var validSourceTypes = map[string]struct{}{
	"gov_primary_record":       {},
	"court_opinion_or_order":   {},
	"third_party_exhibit":      {},
	"historical_public_domain": {},
	"licensed_media":           {},
	"in_house_recreation":      {},
}

var validLicenseStatuses = map[string]struct{}{
	"PUBLIC_DOMAIN": {},
	"LICENSED":      {},
	"RECREATED":     {},
}

var evidenceDBMimeExtensions = map[string]string{
	"application/pdf": "pdf",
	"image/png":       "png",
	"image/jpeg":      "jpg",
	"text/plain":      "txt",
}

// EvidenceWriter defines persistence operations for evidence upload and metadata updates.
type EvidenceWriter interface {
	Create(ctx context.Context, in store.CreateEvidenceInput) (*models.EvidenceItem, error)
	UpdateMetadata(ctx context.Context, id uuid.UUID, in store.UpdateEvidenceInput) (*store.UpdateEvidenceResult, error)
}

// AdminEvidenceWriteHandler serves POST /cases/{id}/evidence and PUT /evidence/{id}.
type AdminEvidenceWriteHandler struct {
	evidence  EvidenceWriter
	storage   storage.Storage
	publisher queue.EvidencePublisher
	logger    zerolog.Logger
}

// NewAdminEvidenceWriteHandler constructs an AdminEvidenceWriteHandler.
func NewAdminEvidenceWriteHandler(
	evidence EvidenceWriter,
	st storage.Storage,
	publisher queue.EvidencePublisher,
	logger zerolog.Logger,
) *AdminEvidenceWriteHandler {
	return &AdminEvidenceWriteHandler{
		evidence:  evidence,
		storage:   st,
		publisher: publisher,
		logger:    logger,
	}
}

type uploadEvidenceResponse struct {
	EvidenceID       uuid.UUID `json:"evidence_id"`
	ProcessingStatus string    `json:"processing_status"`
	JobPublished     bool      `json:"job_published"`
	SHA256Hash       string    `json:"sha256_hash"`
	StorageKey       string    `json:"storage_key"`
}

// Upload handles POST /api/v1/admin/cases/{id}/evidence (multipart/form-data).
// Fail-fast policy checks (Q5, 4x3 matrix recreation tier cap, LICENSED record requirement,
// RECREATED source/note requirements) execute BEFORE touching PostgreSQL or object storage.
func (h *AdminEvidenceWriteHandler) Upload(w http.ResponseWriter, r *http.Request) {
	actorID, ok := adminActorIDFromRequest(w, r)
	if !ok {
		return
	}

	caseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid case id")
		return
	}

	// Bound total multipart body slightly above max file size to accommodate form fields.
	r.Body = http.MaxBytesReader(w, r.Body, maxEvidenceUploadBytes+1024*1024)
	if err := r.ParseMultipartForm(multipartMemoryBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			middleware.WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "uploaded file exceeds 25 MB limit")
			return
		}
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid multipart/form-data payload")
		return
	}
	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
	}

	title := strings.TrimSpace(r.FormValue("title"))
	sourceType := strings.TrimSpace(r.FormValue("source_type"))
	licenseStatus := strings.TrimSpace(r.FormValue("license_status"))
	tierStr := strings.TrimSpace(r.FormValue("verification_tier"))
	reportedAsDateStr := strings.TrimSpace(r.FormValue("reported_as_date"))
	if reportedAsDateStr == "" {
		reportedAsDateStr = strings.TrimSpace(r.FormValue("retrieved_at"))
	}

	if title == "" {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "title is required")
		return
	}
	if _, ok := validSourceTypes[sourceType]; !ok {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid source_type")
		return
	}
	if _, ok := validLicenseStatuses[licenseStatus]; !ok {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "license_status must be PUBLIC_DOMAIN, LICENSED, or RECREATED")
		return
	}

	tierInt, err := strconv.Atoi(tierStr)
	if err != nil || tierInt < 1 || tierInt > 4 {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "verification_tier must be an integer between 1 and 4")
		return
	}
	tier := int16(tierInt)

	retrievedAt, err := parseFlexibleDate(reportedAsDateStr)
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "reported_as_date is required (RFC3339 or YYYY-MM-DD)")
		return
	}

	// Fail-fast policy validation BEFORE hitting storage or PostgreSQL:
	licensingRecIDStr := strings.TrimSpace(r.FormValue("licensing_record_id"))
	recSourceIDsRaw := strings.TrimSpace(r.FormValue("recreation_source_ids"))
	recAuthorNoteStr := strings.TrimSpace(r.FormValue("recreation_author_note"))

	if errCode, errMsg := validateEvidenceIntegrityPolicy(
		sourceType,
		licenseStatus,
		tier,
		licensingRecIDStr,
		recSourceIDsRaw,
		recAuthorNoteStr,
	); errMsg != "" {
		middleware.WriteError(w, r, http.StatusBadRequest, errCode, errMsg)
		return
	}

	var licensingRecordID *uuid.UUID
	if licensingRecIDStr != "" {
		parsedID, err := uuid.Parse(licensingRecIDStr)
		if err != nil {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "licensing_record_id must be a valid UUID")
			return
		}
		licensingRecordID = &parsedID
	}

	var recSourcesJSON json.RawMessage
	var recAuthorNote *string
	if recSourceIDsRaw != "" {
		normalized, err := normalizeRecreationSourceIDs(recSourceIDsRaw)
		if err != nil {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		recSourcesJSON = normalized
	}
	if recAuthorNoteStr != "" {
		recAuthorNote = &recAuthorNoteStr
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "file field is required")
		return
	}
	defer func() { _ = file.Close() }()

	// Determine and validate MIME type against both storage allowlist and DB chk_mime_type.
	contentType := normalizeMIME(header.Header.Get("Content-Type"))
	if contentType == "" || contentType == "application/octet-stream" {
		sniff := make([]byte, 512)
		n, readErr := file.Read(sniff)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "failed to inspect uploaded file")
			return
		}
		contentType = normalizeMIME(http.DetectContentType(sniff[:n]))
		if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
			middleware.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to rewind uploaded file")
			return
		}
	}

	if !storage.IsAllowedContentType(contentType) {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", fmt.Sprintf("unsupported content-type %q", contentType))
		return
	}
	ext, dbSupported := evidenceDBMimeExtensions[contentType]
	if !dbSupported {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", fmt.Sprintf("evidence mime_type %q is not supported by schema (allowed: application/pdf, image/png, image/jpeg, text/plain)", contentType))
		return
	}

	// Compute SHA-256 and exact byte size before constructing the immutable server-generated object key
	// (client filenames are never used in object_key per SECURITY_ARCHITECTURE.md T3).
	hasher := sha256.New()
	limited := io.LimitReader(file, maxEvidenceUploadBytes+1)
	byteSize, err := io.Copy(hasher, limited)
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "failed to read uploaded file")
		return
	}
	if byteSize == 0 {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "uploaded file must not be empty")
		return
	}
	if byteSize > maxEvidenceUploadBytes {
		middleware.WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "uploaded file exceeds 25 MB limit")
		return
	}
	shaHex := hex.EncodeToString(hasher.Sum(nil))

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to rewind uploaded file")
		return
	}

	evidenceID := uuid.New()
	objectKey := fmt.Sprintf("cases/%s/evidence/%s/%s.%s", caseID.String(), evidenceID.String(), shaHex, ext)

	if _, err := h.storage.Put(r.Context(), objectKey, file, contentType); err != nil {
		if errors.Is(err, storage.ErrUnsupportedContentType) || errors.Is(err, storage.ErrInvalidKey) {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		if errors.Is(err, storage.ErrFileTooLarge) {
			middleware.WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", err.Error())
			return
		}
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "object storage unavailable")
		return
	}

	description := strings.TrimSpace(r.FormValue("description"))
	sourceCitation := strings.TrimSpace(r.FormValue("source_citation"))
	if sourceCitation == "" {
		if description != "" {
			sourceCitation = description
		} else {
			sourceCitation = title
		}
	}
	issuingAuthority := strings.TrimSpace(r.FormValue("issuing_authority"))
	if issuingAuthority == "" {
		issuingAuthority = "Unspecified Authority"
	}
	licenseBasisNote := strings.TrimSpace(r.FormValue("license_basis_note"))
	if licenseBasisNote == "" {
		if description != "" {
			licenseBasisNote = description
		} else {
			licenseBasisNote = "Declared " + licenseStatus
		}
	}

	var sourceURL *string
	if rawURL := strings.TrimSpace(r.FormValue("source_url")); rawURL != "" {
		sourceURL = &rawURL
	}
	prohibitedCleared := strings.EqualFold(strings.TrimSpace(r.FormValue("prohibited_content_cleared")), "true")

	created, err := h.evidence.Create(r.Context(), store.CreateEvidenceInput{
		ID:                       evidenceID,
		CaseID:                   caseID,
		Title:                    title,
		SourceType:               sourceType,
		VerificationTier:         tier,
		LicenseStatus:            licenseStatus,
		LicensingRecordID:        licensingRecordID,
		RecreationSourceIDs:      recSourcesJSON,
		RecreationAuthorNote:     recAuthorNote,
		SourceCitation:           sourceCitation,
		SourceURL:                sourceURL,
		IssuingAuthority:         issuingAuthority,
		LicenseBasisNote:         licenseBasisNote,
		MimeType:                 contentType,
		ByteSize:                 byteSize,
		SHA256Hash:               shaHex,
		ObjectKey:                objectKey,
		ProhibitedContentCleared: prohibitedCleared,
		RetrievedAt:              retrievedAt,
		ActorID:                  actorID,
	})
	if err != nil {
		// Clean up stored blob so failed DB writes never leave orphaned files.
		_ = h.storage.Delete(r.Context(), objectKey)

		if errors.Is(err, store.ErrNotFound) {
			middleware.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "case not found")
			return
		}
		if msg, ok := store.PGErrorMessage(err); ok && store.IsPGConstraintOrTriggerError(err) {
			middleware.WriteError(w, r, http.StatusUnprocessableEntity, "EVIDENCE_POLICY_BLOCKED", msg)
			return
		}
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to persist evidence")
		return
	}

	jobPublished := false
	if h.publisher != nil {
		job := queue.EvidenceProcessingJob{
			JobID:      uuid.NewString(),
			EvidenceID: created.ID,
			StorageKey: created.ObjectKey,
			MimeType:   created.MimeType,
			CaseID:     created.CaseID,
		}
		if err := h.publisher.PublishEvidenceJob(r.Context(), job); err != nil {
			h.logger.Error().
				Err(err).
				Str("request_id", middleware.RequestIDFromContext(r.Context())).
				Str("evidence_id", created.ID.String()).
				Msg("failed to publish evidence processing job to redis stream")
			middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to publish evidence processing job")
			return
		}
		jobPublished = true
		h.logger.Info().
			Str("request_id", middleware.RequestIDFromContext(r.Context())).
			Str("evidence_id", created.ID.String()).
			Str("case_id", created.CaseID.String()).
			Str("stream", queue.EvidenceProcessStream).
			Msg("published to stream, consumer pending")
	}

	writeJSON(w, http.StatusAccepted, uploadEvidenceResponse{
		EvidenceID:       created.ID,
		ProcessingStatus: created.ProcessingStatus,
		JobPublished:     jobPublished,
		SHA256Hash:       created.SHA256Hash,
		StorageKey:       created.ObjectKey,
	})
}

type updateEvidenceMetadataRequest struct {
	Title                    *string         `json:"title"`
	Description              *string         `json:"description"`
	SourceType               *string         `json:"source_type"`
	VerificationTier         *int16          `json:"verification_tier"`
	LicenseStatus            *string         `json:"license_status"`
	LicensingRecordID        *string         `json:"licensing_record_id"`
	RecreationSourceIDs      json.RawMessage `json:"recreation_source_ids"`
	RecreationAuthorNote     *string         `json:"recreation_author_note"`
	SourceCitation           *string         `json:"source_citation"`
	SourceURL                *string         `json:"source_url"`
	IssuingAuthority         *string         `json:"issuing_authority"`
	LicenseBasisNote         *string         `json:"license_basis_note"`
	ProhibitedContentCleared *bool           `json:"prohibited_content_cleared"`
}

type updateEvidenceResponse struct {
	Evidence          *models.EvidenceItem `json:"evidence"`
	ReReviewTriggered bool                 `json:"re_review_triggered"`
	ParentCaseStatus  string               `json:"parent_case_status"`
}

// UpdateMetadata handles PUT /api/v1/admin/evidence/{id}.
// Updates metadata fields only (never replaces the stored file). If the parent case is PUBLISHED,
// the store transaction atomically demotes case.status back to IN_REVIEW and clears
// published_legal_review_id, requiring a fresh legal_review_records row before re-publish.
func (h *AdminEvidenceWriteHandler) UpdateMetadata(w http.ResponseWriter, r *http.Request) {
	actorID, ok := adminActorIDFromRequest(w, r)
	if !ok {
		return
	}

	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid evidence id")
		return
	}

	var req updateEvidenceMetadataRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, maxAdminJSONBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON request body")
		return
	}

	if req.SourceType != nil {
		if _, ok := validSourceTypes[*req.SourceType]; !ok {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid source_type")
			return
		}
	}
	if req.LicenseStatus != nil {
		if _, ok := validLicenseStatuses[*req.LicenseStatus]; !ok {
			middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid license_status")
			return
		}
	}
	if req.VerificationTier != nil && (*req.VerificationTier < 1 || *req.VerificationTier > 4) {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "verification_tier must be between 1 and 4")
		return
	}

	// If the update payload specifies both fields of a prohibited combination, fail fast with 400.
	if req.SourceType != nil && req.LicenseStatus != nil && *req.SourceType == "third_party_exhibit" && *req.LicenseStatus == "PUBLIC_DOMAIN" {
		middleware.WriteError(w, r, http.StatusBadRequest, "Q5_VIOLATION", "Q5 violation: third-party exhibits cannot be PUBLIC_DOMAIN. Use LICENSED or cite via a Tier 1 court opinion.")
		return
	}
	if req.VerificationTier != nil && req.LicenseStatus != nil && (*req.VerificationTier == 3 || *req.VerificationTier == 4) && *req.LicenseStatus == "RECREATED" {
		middleware.WriteError(w, r, http.StatusBadRequest, "RECREATED_TIER_TOO_HIGH", "Recreations cannot claim Tier 3+.")
		return
	}

	var licensingRecordID *uuid.UUID
	clearLicensing := false
	if req.LicensingRecordID != nil {
		trimmed := strings.TrimSpace(*req.LicensingRecordID)
		if trimmed == "" {
			clearLicensing = true
		} else {
			parsed, err := uuid.Parse(trimmed)
			if err != nil {
				middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "licensing_record_id must be a valid UUID")
				return
			}
			licensingRecordID = &parsed
		}
	}

	sourceCitation := req.SourceCitation
	if sourceCitation == nil && req.Description != nil {
		sourceCitation = req.Description
	}

	res, err := h.evidence.UpdateMetadata(r.Context(), evidenceID, store.UpdateEvidenceInput{
		Title:                    req.Title,
		SourceType:               req.SourceType,
		VerificationTier:         req.VerificationTier,
		LicenseStatus:            req.LicenseStatus,
		LicensingRecordID:        licensingRecordID,
		ClearLicensingRecordID:   clearLicensing,
		RecreationSourceIDs:      req.RecreationSourceIDs,
		RecreationAuthorNote:     req.RecreationAuthorNote,
		SourceCitation:           sourceCitation,
		SourceURL:                req.SourceURL,
		IssuingAuthority:         req.IssuingAuthority,
		LicenseBasisNote:         req.LicenseBasisNote,
		ProhibitedContentCleared: req.ProhibitedContentCleared,
		ActorID:                  actorID,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			middleware.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "evidence item not found")
			return
		}
		if msg, ok := store.PGErrorMessage(err); ok && store.IsPGConstraintOrTriggerError(err) {
			middleware.WriteError(w, r, http.StatusUnprocessableEntity, "EVIDENCE_POLICY_BLOCKED", msg)
			return
		}
		middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "failed to update evidence metadata")
		return
	}

	writeJSON(w, http.StatusOK, updateEvidenceResponse{
		Evidence:          res.Evidence,
		ReReviewTriggered: res.ReReviewTriggered,
		ParentCaseStatus:  res.ParentCaseStatus,
	})
}

func validateEvidenceIntegrityPolicy(
	sourceType, licenseStatus string,
	tier int16,
	licensingRecordID, recSourceIDsRaw, recAuthorNote string,
) (string, string) {
	// 1. Q5 Hard Rule: third_party_exhibit CANNOT be PUBLIC_DOMAIN.
	if sourceType == "third_party_exhibit" && licenseStatus == "PUBLIC_DOMAIN" {
		return "Q5_VIOLATION", "Q5 violation: third-party exhibits cannot be PUBLIC_DOMAIN. Use LICENSED or cite via a Tier 1 court opinion."
	}
	// 2. 4x3 Matrix Hard Rule (R1): RECREATED cannot claim Tier 3 or Tier 4.
	if (tier == 3 || tier == 4) && licenseStatus == "RECREATED" {
		return "RECREATED_TIER_TOO_HIGH", "Recreations cannot claim Tier 3+."
	}
	// 3. LICENSED requires licensing_record_id.
	if licenseStatus == "LICENSED" && licensingRecordID == "" {
		return "LICENSING_RECORD_REQUIRED", "licensing_record_id is required when license_status is LICENSED"
	}
	// 4. RECREATED requires non-empty recreation_source_ids and recreation_author_note.
	if licenseStatus == "RECREATED" {
		if recSourceIDsRaw == "" {
			return "RECREATION_FIELDS_REQUIRED", "recreation_source_ids is required when license_status is RECREATED"
		}
		if _, err := normalizeRecreationSourceIDs(recSourceIDsRaw); err != nil {
			return "RECREATION_FIELDS_REQUIRED", err.Error()
		}
		if recAuthorNote == "" {
			return "RECREATION_FIELDS_REQUIRED", "recreation_author_note is required when license_status is RECREATED"
		}
	}
	return "", ""
}

func normalizeRecreationSourceIDs(raw string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("recreation_source_ids must be a non-empty JSON array or comma-separated list")
	}

	if strings.HasPrefix(trimmed, "[") {
		var items []string
		if err := json.Unmarshal([]byte(trimmed), &items); err != nil || len(items) == 0 {
			return nil, errors.New("recreation_source_ids must be a non-empty array of strings")
		}
		for _, item := range items {
			if strings.TrimSpace(item) == "" {
				return nil, errors.New("recreation_source_ids must not contain empty entries")
			}
		}
		b, _ := json.Marshal(items)
		return json.RawMessage(b), nil
	}

	parts := strings.Split(trimmed, ",")
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			cleaned = append(cleaned, v)
		}
	}
	if len(cleaned) == 0 {
		return nil, errors.New("recreation_source_ids must contain at least one source id")
	}
	b, _ := json.Marshal(cleaned)
	return json.RawMessage(b), nil
}

func parseFlexibleDate(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, errors.New("empty date")
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, errors.New("invalid date format")
}

func normalizeMIME(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if idx := strings.IndexByte(ct, ';'); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	return ct
}
