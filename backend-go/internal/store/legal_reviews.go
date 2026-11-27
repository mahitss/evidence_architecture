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

// LegalReviewStore queries the immutable legal_review_records table.
type LegalReviewStore struct {
	pool *pgxpool.Pool
}

// GetByID retrieves a legal review record by its UUID primary key.
func (ls *LegalReviewStore) GetByID(ctx context.Context, id uuid.UUID) (*models.LegalReview, error) {
	const query = `
		SELECT
			id,
			case_id,
			reviewer_id,
			external_counsel_ref,
			decision,
			prohibited_checklist_passed,
			genuine_gaps_verified,
			license_audit_passed,
			notes,
			reviewed_at,
			created_at
		FROM legal_review_records
		WHERE id = $1
	`

	var lr models.LegalReview
	err := ls.pool.QueryRow(ctx, query, id).Scan(
		&lr.ID,
		&lr.CaseID,
		&lr.ReviewerID,
		&lr.ExternalCounselRef,
		&lr.Decision,
		&lr.ProhibitedChecklistPassed,
		&lr.GenuineGapsVerified,
		&lr.LicenseAuditPassed,
		&lr.Notes,
		&lr.ReviewedAt,
		&lr.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("get legal review by id %s: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get legal review by id %s: %w", id, err)
	}
	return &lr, nil
}
