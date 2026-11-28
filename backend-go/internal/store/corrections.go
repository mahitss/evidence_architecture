package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
)

// CorrectionStore manages anonymous correction_requests submissions.
type CorrectionStore struct {
	pool  *pgxpool.Pool
	audit *AuditStore
}

// CreateCorrectionInput holds validated fields for POST /api/v1/corrections.
type CreateCorrectionInput struct {
	CaseSlug              string
	EvidenceID            *uuid.UUID
	RequestType           string
	RequesterContactEmail string
	Description           string
	SupportingCitation    *string
}

// Create persists a correction_requests row with status='OPEN' (per frozen DB CHECK constraint)
// and appends an ANONYMOUS audit_log record in the same transaction.
func (cs *CorrectionStore) Create(ctx context.Context, in CreateCorrectionInput) (*models.CorrectionRequest, error) {
	tx, err := cs.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create correction tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var caseID *uuid.UUID
	slug := strings.TrimSpace(in.CaseSlug)
	if slug != "" {
		var foundID uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM cases WHERE slug = $1 AND status = 'PUBLISHED'`, slug).Scan(&foundID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("lookup case slug %q for correction: %w", slug, err)
		}
		if err == nil {
			caseID = &foundID
		}
	}

	reqType := strings.TrimSpace(in.RequestType)
	if reqType == "" {
		reqType = "factual_correction"
	}
	contactEmail := strings.TrimSpace(in.RequesterContactEmail)
	if contactEmail == "" {
		contactEmail = "anonymous@no-contact.invalid"
	}

	ticketID := generateTicketID()

	const insertQuery = `
		INSERT INTO correction_requests (
			ticket_id,
			case_id,
			evidence_id,
			request_type,
			requester_contact_email,
			description,
			supporting_citation,
			status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'OPEN')
		RETURNING
			id,
			ticket_id,
			case_id,
			evidence_id,
			request_type,
			requester_contact_email,
			description,
			supporting_citation,
			status,
			created_at
	`

	var rec models.CorrectionRequest
	if err := tx.QueryRow(
		ctx,
		insertQuery,
		ticketID,
		caseID,
		in.EvidenceID,
		reqType,
		contactEmail,
		strings.TrimSpace(in.Description),
		in.SupportingCitation,
	).Scan(
		&rec.ID,
		&rec.TicketID,
		&rec.CaseID,
		&rec.EvidenceID,
		&rec.RequestType,
		&rec.RequesterContactEmail,
		&rec.Description,
		&rec.SupportingCitation,
		&rec.Status,
		&rec.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("insert correction_requests: %w", err)
	}

	targetID := rec.ID
	if err := cs.audit.Append(ctx, tx, AuditEntry{
		Action:     "correction.submitted",
		ActorRole:  "ANONYMOUS",
		TargetType: "correction_request",
		TargetID:   &targetID,
		Metadata: map[string]any{
			"ticket_id":    rec.TicketID,
			"request_type": rec.RequestType,
			"has_case":     rec.CaseID != nil,
		},
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create correction tx: %w", err)
	}
	return &rec, nil
}

func generateTicketID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("COR-%s-%s", time.Now().UTC().Format("20060102"), strings.ToUpper(hex.EncodeToString(b)))
}
