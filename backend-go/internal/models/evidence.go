package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EvidenceItem represents a row in the evidence_items table.
type EvidenceItem struct {
	ID                       uuid.UUID       `json:"id"`
	CaseID                   uuid.UUID       `json:"case_id"`
	Title                    string          `json:"title"`
	SourceType               string          `json:"source_type"`
	VerificationTier         int16           `json:"verification_tier"`
	LicenseStatus            string          `json:"license_status"`
	LicensingRecordID        *uuid.UUID      `json:"licensing_record_id"`
	RecreationSourceIDs      json.RawMessage `json:"recreation_source_ids,omitempty"`
	RecreationAuthorNote     *string         `json:"recreation_author_note"`
	SourceCitation           string          `json:"source_citation"`
	SourceURL                *string         `json:"source_url"`
	IssuingAuthority         string          `json:"issuing_authority"`
	LicenseBasisNote         string          `json:"license_basis_note"`
	MimeType                 string          `json:"mime_type"`
	ByteSize                 int64           `json:"byte_size"`
	SHA256Hash               string          `json:"sha256_hash"`
	ObjectKey                string          `json:"object_key"`
	ProcessingStatus         string          `json:"processing_status"`
	LastProcessedJobID       *uuid.UUID      `json:"last_processed_job_id"`
	ExtractedText            *string         `json:"extracted_text,omitempty"`
	PageCount                *int            `json:"page_count"`
	OCRUsed                  bool            `json:"ocr_used"`
	ProhibitedContentCleared bool            `json:"prohibited_content_cleared"`
	IsRedacted               bool            `json:"is_redacted"`
	RedactionReason          *string         `json:"redaction_reason"`
	RetrievedAt              time.Time       `json:"retrieved_at"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
}

// EvidenceMetadataItem is the metadata-only view returned in GET /api/v1/admin/cases/{id}
// (omits file bytes / raw extracted text and attaches a short-lived signed storage URL).
type EvidenceMetadataItem struct {
	ID                       uuid.UUID       `json:"id"`
	CaseID                   uuid.UUID       `json:"case_id"`
	Title                    string          `json:"title"`
	SourceType               string          `json:"source_type"`
	VerificationTier         int16           `json:"verification_tier"`
	LicenseStatus            string          `json:"license_status"`
	LicensingRecordID        *uuid.UUID      `json:"licensing_record_id"`
	RecreationSourceIDs      json.RawMessage `json:"recreation_source_ids,omitempty"`
	RecreationAuthorNote     *string         `json:"recreation_author_note"`
	SourceCitation           string          `json:"source_citation"`
	SourceURL                *string         `json:"source_url"`
	IssuingAuthority         string          `json:"issuing_authority"`
	LicenseBasisNote         string          `json:"license_basis_note"`
	MimeType                 string          `json:"mime_type"`
	ByteSize                 int64           `json:"byte_size"`
	SHA256Hash               string          `json:"sha256_hash"`
	ObjectKey                string          `json:"object_key"`
	ProcessingStatus         string          `json:"processing_status"`
	ProhibitedContentCleared bool            `json:"prohibited_content_cleared"`
	IsRedacted               bool            `json:"is_redacted"`
	RedactionReason          *string         `json:"redaction_reason"`
	RetrievedAt              time.Time       `json:"retrieved_at"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
	SignedURL                string          `json:"signed_url"`
}

// ToMetadata converts an EvidenceItem to its metadata-only API view with a signed URL.
func (e *EvidenceItem) ToMetadata(signedURL string) EvidenceMetadataItem {
	return EvidenceMetadataItem{
		ID:                       e.ID,
		CaseID:                   e.CaseID,
		Title:                    e.Title,
		SourceType:               e.SourceType,
		VerificationTier:         e.VerificationTier,
		LicenseStatus:            e.LicenseStatus,
		LicensingRecordID:        e.LicensingRecordID,
		RecreationSourceIDs:      e.RecreationSourceIDs,
		RecreationAuthorNote:     e.RecreationAuthorNote,
		SourceCitation:           e.SourceCitation,
		SourceURL:                e.SourceURL,
		IssuingAuthority:         e.IssuingAuthority,
		LicenseBasisNote:         e.LicenseBasisNote,
		MimeType:                 e.MimeType,
		ByteSize:                 e.ByteSize,
		SHA256Hash:               e.SHA256Hash,
		ObjectKey:                e.ObjectKey,
		ProcessingStatus:         e.ProcessingStatus,
		ProhibitedContentCleared: e.ProhibitedContentCleared,
		IsRedacted:               e.IsRedacted,
		RedactionReason:          e.RedactionReason,
		RetrievedAt:              e.RetrievedAt,
		CreatedAt:                e.CreatedAt,
		UpdatedAt:                e.UpdatedAt,
		SignedURL:                signedURL,
	}
}
