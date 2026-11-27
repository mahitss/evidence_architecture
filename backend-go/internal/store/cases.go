package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
)

// CaseStore queries and mutates the cases table with transactional audit logging.
type CaseStore struct {
	pool  *pgxpool.Pool
	audit *AuditStore
}

// CreateCaseInput holds validated fields for creating a new case.
type CreateCaseInput struct {
	Slug             string
	Title            string
	Category         string
	Jurisdiction     string
	EUOrigin         bool
	GDPRVerified     bool
	Status           string
	OfficialBriefing string
	GenuineGaps      json.RawMessage
	ActorID          uuid.UUID
}

// UpdateCaseInput holds validated mutable fields for updating an existing DRAFT or IN_REVIEW case.
type UpdateCaseInput struct {
	Slug             *string
	Title            *string
	Category         *string
	Jurisdiction     *string
	EUOrigin         *bool
	GDPRVerified     *bool
	OfficialBriefing *string
	GenuineGaps      json.RawMessage
	ActorID          uuid.UUID
}

const caseSelectColumns = `
	id,
	slug,
	title,
	category,
	jurisdiction,
	eu_origin,
	gdpr_verified,
	status,
	official_briefing,
	genuine_gaps,
	published_corrections_log,
	published_legal_review_id,
	search_sync_pending,
	published_at,
	retracted_at,
	retraction_reason,
	created_by,
	updated_by,
	created_at,
	updated_at
`

// GetByID retrieves a case by its UUID primary key.
func (cs *CaseStore) GetByID(ctx context.Context, id uuid.UUID) (*models.Case, error) {
	query := `SELECT` + caseSelectColumns + `FROM cases WHERE id = $1`
	c, err := scanCase(cs.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("get case by id %s: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get case by id %s: %w", id, err)
	}
	return c, nil
}

// GetBySlug retrieves a case by its unique slug.
func (cs *CaseStore) GetBySlug(ctx context.Context, slug string) (*models.Case, error) {
	query := `SELECT` + caseSelectColumns + `FROM cases WHERE slug = $1`
	c, err := scanCase(cs.pool.QueryRow(ctx, query, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("get case by slug %q: %w", slug, ErrNotFound)
		}
		return nil, fmt.Errorf("get case by slug %q: %w", slug, err)
	}
	return c, nil
}

// Create inserts a new case and appends a 'case.created' audit_log row in the same transaction.
func (cs *CaseStore) Create(ctx context.Context, in CreateCaseInput) (*models.Case, error) {
	tx, err := cs.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create case tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	jurisdiction := in.Jurisdiction
	if jurisdiction == "" {
		jurisdiction = "US"
	}
	status := in.Status
	if status == "" {
		status = "DRAFT"
	}
	gaps := in.GenuineGaps
	if len(gaps) == 0 {
		gaps = json.RawMessage(`[]`)
	}

	const insertQuery = `
		INSERT INTO cases (
			slug,
			title,
			category,
			jurisdiction,
			eu_origin,
			gdpr_verified,
			status,
			official_briefing,
			genuine_gaps,
			created_by,
			updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $10)
		RETURNING` + caseSelectColumns

	c, err := scanCase(tx.QueryRow(
		ctx,
		insertQuery,
		in.Slug,
		in.Title,
		in.Category,
		jurisdiction,
		in.EUOrigin,
		in.GDPRVerified,
		status,
		in.OfficialBriefing,
		string(gaps),
		in.ActorID,
	))
	if err != nil {
		if IsPGUniqueViolation(err) {
			return nil, fmt.Errorf("slug %q already exists: %w", in.Slug, ErrConflict)
		}
		if IsPGConstraintOrTriggerError(err) {
			return nil, fmt.Errorf("create case rejected: %w", err)
		}
		return nil, fmt.Errorf("insert case: %w", err)
	}

	actorID := in.ActorID
	caseID := c.ID
	if err := cs.audit.Append(ctx, tx, AuditEntry{
		Action:     "case.created",
		ActorID:    &actorID,
		TargetType: "case",
		TargetID:   &caseID,
		Metadata: map[string]any{
			"slug":     c.Slug,
			"category": c.Category,
			"status":   c.Status,
		},
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create case tx: %w", err)
	}
	return c, nil
}

// Update modifies a DRAFT or IN_REVIEW case and appends a 'case.updated' audit_log row in the same transaction.
// Attempting to edit APPROVED or PUBLISHED cases returns ErrConflict.
func (cs *CaseStore) Update(ctx context.Context, id uuid.UUID, in UpdateCaseInput) (*models.Case, error) {
	tx, err := cs.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update case tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := scanCase(tx.QueryRow(ctx, `SELECT`+caseSelectColumns+`FROM cases WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("case %s not found: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("lock case %s: %w", id, err)
	}

	if current.Status != "DRAFT" && current.Status != "IN_REVIEW" {
		return nil, fmt.Errorf("case must be retracted to DRAFT to edit: %w", ErrConflict)
	}

	slug := current.Slug
	if in.Slug != nil {
		slug = *in.Slug
	}
	title := current.Title
	if in.Title != nil {
		title = *in.Title
	}
	category := current.Category
	if in.Category != nil {
		category = *in.Category
	}
	jurisdiction := current.Jurisdiction
	if in.Jurisdiction != nil {
		jurisdiction = *in.Jurisdiction
	}
	euOrigin := current.EUOrigin
	if in.EUOrigin != nil {
		euOrigin = *in.EUOrigin
	}
	gdprVerified := current.GDPRVerified
	if in.GDPRVerified != nil {
		gdprVerified = *in.GDPRVerified
	}
	briefing := current.OfficialBriefing
	if in.OfficialBriefing != nil {
		briefing = *in.OfficialBriefing
	}
	gaps := current.GenuineGaps
	if len(in.GenuineGaps) > 0 {
		gaps = in.GenuineGaps
	}

	const updateQuery = `
		UPDATE cases
		SET
			slug = $2,
			title = $3,
			category = $4,
			jurisdiction = $5,
			eu_origin = $6,
			gdpr_verified = $7,
			official_briefing = $8,
			genuine_gaps = $9::jsonb,
			updated_by = $10,
			updated_at = NOW()
		WHERE id = $1
		RETURNING` + caseSelectColumns

	updated, err := scanCase(tx.QueryRow(
		ctx,
		updateQuery,
		id,
		slug,
		title,
		category,
		jurisdiction,
		euOrigin,
		gdprVerified,
		briefing,
		string(gaps),
		in.ActorID,
	))
	if err != nil {
		if IsPGUniqueViolation(err) {
			return nil, fmt.Errorf("slug %q already exists: %w", slug, ErrConflict)
		}
		return nil, fmt.Errorf("update case %s: %w", id, err)
	}

	actorID := in.ActorID
	caseID := updated.ID
	if err := cs.audit.Append(ctx, tx, AuditEntry{
		Action:     "case.updated",
		ActorID:    &actorID,
		TargetType: "case",
		TargetID:   &caseID,
		Metadata: map[string]any{
			"slug":   updated.Slug,
			"status": updated.Status,
		},
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update case tx: %w", err)
	}
	return updated, nil
}

// Publish attempts to transition a case to PUBLISHED without bypassing the PostgreSQL trigger
// enforce_legal_review_before_publish. Any trigger rejection is returned unwrapped so the handler
// surfaces the exact PostgreSQL exception with HTTP 422.
func (cs *CaseStore) Publish(ctx context.Context, id, actorID uuid.UUID) (*models.Case, error) {
	tx, err := cs.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin publish case tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const publishQuery = `
		UPDATE cases
		SET
			status = 'PUBLISHED',
			updated_by = $2,
			updated_at = NOW()
		WHERE id = $1
		RETURNING` + caseSelectColumns

	published, err := scanCase(tx.QueryRow(ctx, publishQuery, id, actorID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("case %s not found: %w", id, ErrNotFound)
		}
		return nil, err
	}

	caseID := published.ID
	if err := cs.audit.Append(ctx, tx, AuditEntry{
		Action:     "case.published",
		ActorID:    &actorID,
		TargetType: "case",
		TargetID:   &caseID,
		Metadata: map[string]any{
			"slug":                      published.Slug,
			"published_legal_review_id": published.PublishedLegalReviewID,
		},
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit publish case tx: %w", err)
	}
	return published, nil
}

// Retract transitions a PUBLISHED case to RETRACTED with a mandatory reason and writes 'case.retracted' to audit_log.
func (cs *CaseStore) Retract(ctx context.Context, id, actorID uuid.UUID, reason string) (*models.Case, error) {
	tx, err := cs.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin retract case tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := scanCase(tx.QueryRow(ctx, `SELECT`+caseSelectColumns+`FROM cases WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("case %s not found: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("lock case %s: %w", id, err)
	}

	if current.Status != "PUBLISHED" {
		return nil, fmt.Errorf("only PUBLISHED cases can be retracted (current status=%s): %w", current.Status, ErrConflict)
	}

	const retractQuery = `
		UPDATE cases
		SET
			status = 'RETRACTED',
			retracted_at = NOW(),
			retraction_reason = $2,
			updated_by = $3,
			updated_at = NOW()
		WHERE id = $1
		RETURNING` + caseSelectColumns

	retracted, err := scanCase(tx.QueryRow(ctx, retractQuery, id, reason, actorID))
	if err != nil {
		return nil, fmt.Errorf("retract case %s: %w", id, err)
	}

	caseID := retracted.ID
	reasonCopy := reason
	if err := cs.audit.Append(ctx, tx, AuditEntry{
		Action:     "case.retracted",
		ActorID:    &actorID,
		TargetType: "case",
		TargetID:   &caseID,
		ReasonCode: &reasonCopy,
		Metadata: map[string]any{
			"slug":   retracted.Slug,
			"reason": reason,
		},
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit retract case tx: %w", err)
	}
	return retracted, nil
}

func scanCase(row pgx.Row) (*models.Case, error) {
	var c models.Case
	err := row.Scan(
		&c.ID,
		&c.Slug,
		&c.Title,
		&c.Category,
		&c.Jurisdiction,
		&c.EUOrigin,
		&c.GDPRVerified,
		&c.Status,
		&c.OfficialBriefing,
		&c.GenuineGaps,
		&c.PublishedCorrectionsLog,
		&c.PublishedLegalReviewID,
		&c.SearchSyncPending,
		&c.PublishedAt,
		&c.RetractedAt,
		&c.RetractionReason,
		&c.CreatedBy,
		&c.UpdatedBy,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
