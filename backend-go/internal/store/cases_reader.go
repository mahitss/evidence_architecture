package store

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
)

// PublishedCaseSummary is the read-side summary returned by GET /api/v1/cases.
type PublishedCaseSummary struct {
	ID            uuid.UUID `json:"id"`
	Slug          string    `json:"slug"`
	Title         string    `json:"title"`
	Summary       string    `json:"summary"`
	Category      string    `json:"category"`
	Jurisdiction  string    `json:"jurisdiction"`
	PublishedAt   time.Time `json:"published_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	EvidenceCount int       `json:"evidence_count"`
}

// ErrInvalidCursor indicates a malformed pagination cursor.
var ErrInvalidCursor = errors.New("invalid cursor")

// EncodeCaseCursor builds an opaque base64url cursor from (published_at, id).
func EncodeCaseCursor(publishedAt time.Time, id uuid.UUID) string {
	raw := publishedAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCaseCursor parses an opaque base64url cursor created by EncodeCaseCursor.
func DecodeCaseCursor(cursor string) (time.Time, uuid.UUID, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	return ts.UTC(), id, nil
}

// ListPublished returns cursor-paginated PUBLISHED cases ordered by (published_at DESC, id DESC).
// Strictly filters status = 'PUBLISHED' in SQL so drafts/retracted cases can never leak.
func (cs *CaseStore) ListPublished(
	ctx context.Context,
	category string,
	cursor string,
	limit int,
) ([]PublishedCaseSummary, *string, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	args := make([]any, 0, 4)
	where := []string{"c.status = 'PUBLISHED'", "c.published_at IS NOT NULL"}

	if category != "" {
		args = append(args, category)
		where = append(where, fmt.Sprintf("c.category = $%d", len(args)))
	}

	if cursor != "" {
		ts, id, err := DecodeCaseCursor(cursor)
		if err != nil {
			return nil, nil, false, err
		}
		args = append(args, ts, id)
		where = append(where, fmt.Sprintf("(c.published_at, c.id) < ($%d, $%d)", len(args)-1, len(args)))
	}

	args = append(args, limit+1)
	query := fmt.Sprintf(`
		SELECT
			c.id,
			c.slug,
			c.title,
			c.official_briefing,
			c.category,
			c.jurisdiction,
			c.published_at,
			c.updated_at,
			(SELECT COUNT(*) FROM evidence_items e WHERE e.case_id = c.id) AS evidence_count
		FROM cases c
		WHERE %s
		ORDER BY c.published_at DESC, c.id DESC
		LIMIT $%d
	`, strings.Join(where, " AND "), len(args))

	rows, err := cs.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, false, fmt.Errorf("list published cases: %w", err)
	}
	defer rows.Close()

	items := make([]PublishedCaseSummary, 0, limit)
	for rows.Next() {
		var s PublishedCaseSummary
		if err := rows.Scan(
			&s.ID,
			&s.Slug,
			&s.Title,
			&s.Summary,
			&s.Category,
			&s.Jurisdiction,
			&s.PublishedAt,
			&s.UpdatedAt,
			&s.EvidenceCount,
		); err != nil {
			return nil, nil, false, fmt.Errorf("scan published case summary: %w", err)
		}
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, false, fmt.Errorf("iterate published cases: %w", err)
	}

	hasMore := false
	var nextCursor *string
	if len(items) > limit {
		hasMore = true
		items = items[:limit]
		last := items[len(items)-1]
		encoded := EncodeCaseCursor(last.PublishedAt, last.ID)
		nextCursor = &encoded
	}

	return items, nextCursor, hasMore, nil
}

// GetPublishedBySlug retrieves a case by slug ONLY when status = 'PUBLISHED'.
// Draft, IN_REVIEW, APPROVED, RETRACTED, and missing slugs all return ErrNotFound (T1).
func (cs *CaseStore) GetPublishedBySlug(ctx context.Context, slug string) (*models.Case, error) {
	query := `SELECT` + caseSelectColumns + `FROM cases WHERE slug = $1 AND status = 'PUBLISHED'`
	c, err := scanCase(cs.pool.QueryRow(ctx, query, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("published case %q not found: %w", slug, ErrNotFound)
		}
		return nil, fmt.Errorf("get published case %q: %w", slug, err)
	}
	return c, nil
}

// ListTimelineByCaseID returns ordered timeline events for a case.
func (cs *CaseStore) ListTimelineByCaseID(ctx context.Context, caseID uuid.UUID) ([]models.TimelineEvent, error) {
	const query = `
		SELECT
			id,
			case_id,
			to_char(event_start_date, 'YYYY-MM-DD'),
			CASE WHEN event_end_date IS NOT NULL THEN to_char(event_end_date, 'YYYY-MM-DD') ELSE NULL END,
			sort_order,
			event_type,
			title,
			summary,
			verification_tier,
			linked_evidence_ids,
			has_conflict,
			conflicting_sources,
			controlling_status_note,
			created_at,
			updated_at
		FROM case_timeline_events
		WHERE case_id = $1
		ORDER BY event_start_date ASC, sort_order ASC, id ASC
	`
	rows, err := cs.pool.Query(ctx, query, caseID)
	if err != nil {
		return nil, fmt.Errorf("list timeline events for case %s: %w", caseID, err)
	}
	defer rows.Close()

	events := make([]models.TimelineEvent, 0)
	for rows.Next() {
		var ev models.TimelineEvent
		if err := rows.Scan(
			&ev.ID,
			&ev.CaseID,
			&ev.EventStartDate,
			&ev.EventEndDate,
			&ev.SortOrder,
			&ev.EventType,
			&ev.Title,
			&ev.Summary,
			&ev.VerificationTier,
			&ev.LinkedEvidenceIDs,
			&ev.HasConflict,
			&ev.ConflictingSources,
			&ev.ControllingStatusNote,
			&ev.CreatedAt,
			&ev.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan timeline event: %w", err)
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate timeline events: %w", err)
	}
	return events, nil
}

// GetPublishedByID retrieves an evidence item ONLY if its parent case has status = 'PUBLISHED'.
// Returns ErrNotFound if the evidence item does not exist or its parent case is not PUBLISHED.
func (es *EvidenceStore) GetPublishedByID(ctx context.Context, id uuid.UUID) (*models.EvidenceItem, error) {
	const query = `
		SELECT
			e.id,
			e.case_id,
			e.title,
			e.source_type,
			e.verification_tier,
			e.license_status,
			e.licensing_record_id,
			e.recreation_source_ids,
			e.recreation_author_note,
			e.source_citation,
			e.source_url,
			e.issuing_authority,
			e.license_basis_note,
			e.mime_type,
			e.byte_size,
			e.sha256_hash,
			e.object_key,
			e.processing_status,
			e.last_processed_job_id,
			e.extracted_text,
			e.page_count,
			e.ocr_used,
			e.prohibited_content_cleared,
			e.is_redacted,
			e.redaction_reason,
			e.retrieved_at,
			e.created_at,
			e.updated_at
		FROM evidence_items e
		INNER JOIN cases c ON c.id = e.case_id
		WHERE e.id = $1 AND c.status = 'PUBLISHED'
	`
	ev, err := scanEvidence(es.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("published evidence %s not found: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get published evidence %s: %w", id, err)
	}
	return ev, nil
}

// PublishWithHook transitions a case to PUBLISHED (enforced by PostgreSQL trigger),
// appends the audit_log entry, and invokes postUpdateFn inside the transaction BEFORE commit.
// If postUpdateFn (e.g., synchronous Meilisearch indexing) returns an error, the transaction
// rolls back cleanly so unpublished search unavailability prevents partial publish.
func (cs *CaseStore) PublishWithHook(
	ctx context.Context,
	id, actorID uuid.UUID,
	postUpdateFn func(ctx context.Context, published *models.Case, items []*models.EvidenceItem) error,
) (*models.Case, error) {
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

	if postUpdateFn != nil {
		evRows, err := tx.Query(ctx, `SELECT`+evidenceSelectColumns+`FROM evidence_items WHERE case_id = $1 ORDER BY created_at ASC, id ASC`, published.ID)
		if err != nil {
			return nil, fmt.Errorf("query evidence for search indexing: %w", err)
		}
		items := make([]*models.EvidenceItem, 0)
		for evRows.Next() {
			ev, scanErr := scanEvidence(evRows)
			if scanErr != nil {
				evRows.Close()
				return nil, fmt.Errorf("scan evidence for search indexing: %w", scanErr)
			}
			items = append(items, ev)
		}
		evRows.Close()
		if err := evRows.Err(); err != nil {
			return nil, fmt.Errorf("iterate evidence for search indexing: %w", err)
		}

		if err := postUpdateFn(ctx, published, items); err != nil {
			return nil, fmt.Errorf("post-publish search index failed: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit publish case tx: %w", err)
	}
	return published, nil
}
