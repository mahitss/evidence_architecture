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

// LegalReviewStore queries and creates immutable legal_review_records rows.
type LegalReviewStore struct {
	pool  *pgxpool.Pool
	audit *AuditStore
}

// CreateLegalReviewInput holds parameters for recording an immutable external legal review.
type CreateLegalReviewInput struct {
	CaseID                    uuid.UUID
	ReviewerID                uuid.UUID
	ExternalCounselRef        string
	Decision                  string
	ProhibitedChecklistPassed bool
	GenuineGapsVerified       bool
	LicenseAuditPassed        bool
	Notes                     string
}

const legalReviewSelectColumns = `
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
`

// GetByID retrieves a legal review record by its UUID primary key.
func (ls *LegalReviewStore) GetByID(ctx context.Context, id uuid.UUID) (*models.LegalReview, error) {
	query := `SELECT` + legalReviewSelectColumns + `FROM legal_review_records WHERE id = $1`
	lr, err := scanLegalReview(ls.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("get legal review by id %s: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get legal review by id %s: %w", id, err)
	}
	return lr, nil
}

// Create inserts an immutable legal_review_records row, links cases.published_legal_review_id
// when decision == 'APPROVED', leaves cases.status as DRAFT when decision == 'REJECTED',
// and appends a 'legal_review.created' audit_log row in the same transaction.
func (ls *LegalReviewStore) Create(ctx context.Context, in CreateLegalReviewInput) (*models.LegalReview, error) {
	tx, err := ls.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin legal review tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cases WHERE id = $1 FOR UPDATE)`, in.CaseID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check case %s: %w", in.CaseID, err)
	}
	if !exists {
		return nil, fmt.Errorf("case %s not found: %w", in.CaseID, ErrNotFound)
	}

	const insertQuery = `
		INSERT INTO legal_review_records (
			case_id,
			reviewer_id,
			external_counsel_ref,
			decision,
			prohibited_checklist_passed,
			genuine_gaps_verified,
			license_audit_passed,
			notes,
			reviewed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		RETURNING` + legalReviewSelectColumns

	lr, err := scanLegalReview(tx.QueryRow(
		ctx,
		insertQuery,
		in.CaseID,
		in.ReviewerID,
		in.ExternalCounselRef,
		in.Decision,
		in.ProhibitedChecklistPassed,
		in.GenuineGapsVerified,
		in.LicenseAuditPassed,
		in.Notes,
	))
	if err != nil {
		return nil, err
	}

	if lr.Decision == "APPROVED" {
		if _, err := tx.Exec(
			ctx,
			`UPDATE cases
			 SET published_legal_review_id = $2,
			     updated_by = $3,
			     updated_at = NOW()
			 WHERE id = $1`,
			lr.CaseID,
			lr.ID,
			in.ReviewerID,
		); err != nil {
			return nil, fmt.Errorf("link published_legal_review_id on case %s: %w", lr.CaseID, err)
		}
	}

	actorID := in.ReviewerID
	reviewID := lr.ID
	if err := ls.audit.Append(ctx, tx, AuditEntry{
		Action:     "legal_review.created",
		ActorID:    &actorID,
		TargetType: "legal_review",
		TargetID:   &reviewID,
		Metadata: map[string]any{
			"case_id":                     lr.CaseID,
			"decision":                    lr.Decision,
			"external_counsel_ref":        lr.ExternalCounselRef,
			"prohibited_checklist_passed": lr.ProhibitedChecklistPassed,
		},
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit legal review tx: %w", err)
	}
	return lr, nil
}

func scanLegalReview(row pgx.Row) (*models.LegalReview, error) {
	var lr models.LegalReview
	err := row.Scan(
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
		return nil, err
	}
	return &lr, nil
}
