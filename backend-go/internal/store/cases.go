package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
)

// CaseStore queries the cases table.
type CaseStore struct {
	pool *pgxpool.Pool
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
