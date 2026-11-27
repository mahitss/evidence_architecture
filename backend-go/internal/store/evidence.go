package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
)

// EvidenceStore manages evidence_items reads and transactional writes with audit_log entries.
type EvidenceStore struct {
	pool  *pgxpool.Pool
	audit *AuditStore
}

// CreateEvidenceInput contains validated metadata and file integrity fields for a new evidence item.
type CreateEvidenceInput struct {
	ID                       uuid.UUID
	CaseID                   uuid.UUID
	Title                    string
	SourceType               string
	VerificationTier         int16
	LicenseStatus            string
	LicensingRecordID        *uuid.UUID
	RecreationSourceIDs      json.RawMessage
	RecreationAuthorNote     *string
	SourceCitation           string
	SourceURL                *string
	IssuingAuthority         string
	LicenseBasisNote         string
	MimeType                 string
	ByteSize                 int64
	SHA256Hash               string
	ObjectKey                string
	ProhibitedContentCleared bool
	RetrievedAt              time.Time
	ActorID                  uuid.UUID
}

// UpdateEvidenceInput holds mutable metadata fields for PUT /api/v1/admin/evidence/{id}.
type UpdateEvidenceInput struct {
	Title                    *string
	SourceType               *string
	VerificationTier         *int16
	LicenseStatus            *string
	LicensingRecordID        *uuid.UUID
	ClearLicensingRecordID   bool
	RecreationSourceIDs      json.RawMessage
	RecreationAuthorNote     *string
	SourceCitation           *string
	SourceURL                *string
	IssuingAuthority         *string
	LicenseBasisNote         *string
	ProhibitedContentCleared *bool
	ActorID                  uuid.UUID
}

// UpdateEvidenceResult returns the updated evidence item and whether its parent case was demoted to IN_REVIEW.
type UpdateEvidenceResult struct {
	Evidence          *models.EvidenceItem
	ReReviewTriggered bool
	ParentCaseStatus  string
}

const evidenceSelectColumns = `
	id,
	case_id,
	title,
	source_type,
	verification_tier,
	license_status,
	licensing_record_id,
	recreation_source_ids,
	recreation_author_note,
	source_citation,
	source_url,
	issuing_authority,
	license_basis_note,
	mime_type,
	byte_size,
	sha256_hash,
	object_key,
	processing_status,
	last_processed_job_id,
	extracted_text,
	page_count,
	ocr_used,
	prohibited_content_cleared,
	is_redacted,
	redaction_reason,
	retrieved_at,
	created_at,
	updated_at
`

// GetByID retrieves a single evidence item by UUID.
func (es *EvidenceStore) GetByID(ctx context.Context, id uuid.UUID) (*models.EvidenceItem, error) {
	query := `SELECT` + evidenceSelectColumns + `FROM evidence_items WHERE id = $1`
	ev, err := scanEvidence(es.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("evidence %s not found: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get evidence %s: %w", id, err)
	}
	return ev, nil
}

// ListByCaseID returns all evidence items belonging to caseID ordered by created_at ASC.
func (es *EvidenceStore) ListByCaseID(ctx context.Context, caseID uuid.UUID) ([]*models.EvidenceItem, error) {
	query := `SELECT` + evidenceSelectColumns + `FROM evidence_items WHERE case_id = $1 ORDER BY created_at ASC, id ASC`
	rows, err := es.pool.Query(ctx, query, caseID)
	if err != nil {
		return nil, fmt.Errorf("list evidence for case %s: %w", caseID, err)
	}
	defer rows.Close()

	items := make([]*models.EvidenceItem, 0)
	for rows.Next() {
		ev, err := scanEvidence(rows)
		if err != nil {
			return nil, fmt.Errorf("scan evidence row: %w", err)
		}
		items = append(items, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate evidence rows: %w", err)
	}
	return items, nil
}

// Create inserts a PENDING evidence_items row and appends an 'evidence.created' audit_log row in one transaction.
func (es *EvidenceStore) Create(ctx context.Context, in CreateEvidenceInput) (*models.EvidenceItem, error) {
	tx, err := es.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create evidence tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cases WHERE id = $1)`, in.CaseID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check parent case %s: %w", in.CaseID, err)
	}
	if !exists {
		return nil, fmt.Errorf("case %s not found: %w", in.CaseID, ErrNotFound)
	}

	var recSources any
	if len(in.RecreationSourceIDs) > 0 {
		recSources = string(in.RecreationSourceIDs)
	}

	const insertQuery = `
		INSERT INTO evidence_items (
			id,
			case_id,
			title,
			source_type,
			verification_tier,
			license_status,
			licensing_record_id,
			recreation_source_ids,
			recreation_author_note,
			source_citation,
			source_url,
			issuing_authority,
			license_basis_note,
			mime_type,
			byte_size,
			sha256_hash,
			object_key,
			processing_status,
			prohibited_content_cleared,
			retrieved_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, $11, $12, $13, $14, $15, $16, $17, 'PENDING', $18, $19
		)
		RETURNING` + evidenceSelectColumns

	ev, err := scanEvidence(tx.QueryRow(
		ctx,
		insertQuery,
		in.ID,
		in.CaseID,
		in.Title,
		in.SourceType,
		in.VerificationTier,
		in.LicenseStatus,
		in.LicensingRecordID,
		recSources,
		in.RecreationAuthorNote,
		in.SourceCitation,
		in.SourceURL,
		in.IssuingAuthority,
		in.LicenseBasisNote,
		in.MimeType,
		in.ByteSize,
		in.SHA256Hash,
		in.ObjectKey,
		in.ProhibitedContentCleared,
		in.RetrievedAt,
	))
	if err != nil {
		return nil, err
	}

	actorID := in.ActorID
	evID := ev.ID
	sha := ev.SHA256Hash
	if err := es.audit.Append(ctx, tx, AuditEntry{
		Action:        "evidence.created",
		ActorID:       &actorID,
		TargetType:    "evidence_item",
		TargetID:      &evID,
		PayloadSHA256: &sha,
		Metadata: map[string]any{
			"case_id":           ev.CaseID,
			"source_type":       ev.SourceType,
			"license_status":    ev.LicenseStatus,
			"verification_tier": ev.VerificationTier,
			"object_key":        ev.ObjectKey,
		},
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create evidence tx: %w", err)
	}
	return ev, nil
}

// UpdateMetadata updates mutable metadata fields on an evidence item (never the stored file).
// Per SECURITY_ARCHITECTURE.md T6 & Prompt 8c Phase E: if the parent case is currently PUBLISHED,
// modifying evidence atomically transitions the parent case back to IN_REVIEW and clears
// published_legal_review_id so a fresh legal_review_records row is mandatory before re-publishing.
func (es *EvidenceStore) UpdateMetadata(ctx context.Context, id uuid.UUID, in UpdateEvidenceInput) (*UpdateEvidenceResult, error) {
	tx, err := es.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update evidence tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := scanEvidence(tx.QueryRow(ctx, `SELECT`+evidenceSelectColumns+`FROM evidence_items WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("evidence %s not found: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("lock evidence %s: %w", id, err)
	}

	var parentStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM cases WHERE id = $1 FOR UPDATE`, current.CaseID).Scan(&parentStatus); err != nil {
		return nil, fmt.Errorf("lock parent case %s: %w", current.CaseID, err)
	}

	title := current.Title
	if in.Title != nil {
		title = *in.Title
	}
	sourceType := current.SourceType
	if in.SourceType != nil {
		sourceType = *in.SourceType
	}
	tier := current.VerificationTier
	if in.VerificationTier != nil {
		tier = *in.VerificationTier
	}
	licenseStatus := current.LicenseStatus
	if in.LicenseStatus != nil {
		licenseStatus = *in.LicenseStatus
	}
	licensingRecordID := current.LicensingRecordID
	if in.ClearLicensingRecordID {
		licensingRecordID = nil
	} else if in.LicensingRecordID != nil {
		licensingRecordID = in.LicensingRecordID
	}
	recSources := current.RecreationSourceIDs
	if len(in.RecreationSourceIDs) > 0 {
		recSources = in.RecreationSourceIDs
	}
	var recSourcesArg any
	if len(recSources) > 0 {
		recSourcesArg = string(recSources)
	}
	recAuthorNote := current.RecreationAuthorNote
	if in.RecreationAuthorNote != nil {
		recAuthorNote = in.RecreationAuthorNote
	}
	sourceCitation := current.SourceCitation
	if in.SourceCitation != nil {
		sourceCitation = *in.SourceCitation
	}
	sourceURL := current.SourceURL
	if in.SourceURL != nil {
		sourceURL = in.SourceURL
	}
	issuingAuthority := current.IssuingAuthority
	if in.IssuingAuthority != nil {
		issuingAuthority = *in.IssuingAuthority
	}
	licenseBasisNote := current.LicenseBasisNote
	if in.LicenseBasisNote != nil {
		licenseBasisNote = *in.LicenseBasisNote
	}
	prohibitedCleared := current.ProhibitedContentCleared
	if in.ProhibitedContentCleared != nil {
		prohibitedCleared = *in.ProhibitedContentCleared
	}

	const updateQuery = `
		UPDATE evidence_items
		SET
			title = $2,
			source_type = $3,
			verification_tier = $4,
			license_status = $5,
			licensing_record_id = $6,
			recreation_source_ids = $7::jsonb,
			recreation_author_note = $8,
			source_citation = $9,
			source_url = $10,
			issuing_authority = $11,
			license_basis_note = $12,
			prohibited_content_cleared = $13,
			updated_at = NOW()
		WHERE id = $1
		RETURNING` + evidenceSelectColumns

	updated, err := scanEvidence(tx.QueryRow(
		ctx,
		updateQuery,
		id,
		title,
		sourceType,
		tier,
		licenseStatus,
		licensingRecordID,
		recSourcesArg,
		recAuthorNote,
		sourceCitation,
		sourceURL,
		issuingAuthority,
		licenseBasisNote,
		prohibitedCleared,
	))
	if err != nil {
		return nil, err
	}

	// SECURITY INVARIANT (T6): If parent case was PUBLISHED, editing evidence metadata MUST force
	// the case back to IN_REVIEW and clear published_legal_review_id so a new legal_review_records
	// sign-off is required before the case can be re-published.
	reReviewTriggered := false
	newParentStatus := parentStatus
	if parentStatus == "PUBLISHED" {
		if _, err := tx.Exec(
			ctx,
			`UPDATE cases
			 SET status = 'IN_REVIEW',
			     published_legal_review_id = NULL,
			     published_at = NULL,
			     updated_by = $2,
			     updated_at = NOW()
			 WHERE id = $1`,
			updated.CaseID,
			in.ActorID,
		); err != nil {
			return nil, fmt.Errorf("demote published case %s to IN_REVIEW: %w", updated.CaseID, err)
		}
		reReviewTriggered = true
		newParentStatus = "IN_REVIEW"
	}

	actorID := in.ActorID
	evID := updated.ID
	if err := es.audit.Append(ctx, tx, AuditEntry{
		Action:     "evidence.updated",
		ActorID:    &actorID,
		TargetType: "evidence_item",
		TargetID:   &evID,
		Metadata: map[string]any{
			"case_id":             updated.CaseID,
			"re_review_triggered": reReviewTriggered,
			"parent_case_status":  newParentStatus,
			"source_type":         updated.SourceType,
			"license_status":      updated.LicenseStatus,
			"verification_tier":   updated.VerificationTier,
		},
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update evidence tx: %w", err)
	}

	return &UpdateEvidenceResult{
		Evidence:          updated,
		ReReviewTriggered: reReviewTriggered,
		ParentCaseStatus:  newParentStatus,
	}, nil
}

func scanEvidence(row pgx.Row) (*models.EvidenceItem, error) {
	var e models.EvidenceItem
	err := row.Scan(
		&e.ID,
		&e.CaseID,
		&e.Title,
		&e.SourceType,
		&e.VerificationTier,
		&e.LicenseStatus,
		&e.LicensingRecordID,
		&e.RecreationSourceIDs,
		&e.RecreationAuthorNote,
		&e.SourceCitation,
		&e.SourceURL,
		&e.IssuingAuthority,
		&e.LicenseBasisNote,
		&e.MimeType,
		&e.ByteSize,
		&e.SHA256Hash,
		&e.ObjectKey,
		&e.ProcessingStatus,
		&e.LastProcessedJobID,
		&e.ExtractedText,
		&e.PageCount,
		&e.OCRUsed,
		&e.ProhibitedContentCleared,
		&e.IsRedacted,
		&e.RedactionReason,
		&e.RetrievedAt,
		&e.CreatedAt,
		&e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
