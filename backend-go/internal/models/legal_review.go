package models

import (
	"time"

	"github.com/google/uuid"
)

// LegalReview represents an immutable row in legal_review_records.
type LegalReview struct {
	ID                        uuid.UUID `json:"id"`
	CaseID                    uuid.UUID `json:"case_id"`
	ReviewerID                uuid.UUID `json:"reviewer_id"`
	ExternalCounselRef        string    `json:"external_counsel_ref"`
	Decision                  string    `json:"decision"`
	ProhibitedChecklistPassed bool      `json:"prohibited_checklist_passed"`
	GenuineGapsVerified       bool      `json:"genuine_gaps_verified"`
	LicenseAuditPassed        bool      `json:"license_audit_passed"`
	Notes                     string    `json:"notes"`
	ReviewedAt                time.Time `json:"reviewed_at"`
	CreatedAt                 time.Time `json:"created_at"`
}

// LegalReviewSummary is the minimal projection expanded on GET /api/v1/admin/cases/{id}.
type LegalReviewSummary struct {
	ID                        uuid.UUID `json:"id"`
	ReviewerID                uuid.UUID `json:"reviewer_id"`
	ExternalCounselRef        string    `json:"external_counsel_ref"`
	Decision                  string    `json:"decision"`
	ProhibitedChecklistPassed bool      `json:"prohibited_checklist_passed"`
	ReviewedAt                time.Time `json:"reviewed_at"`
}

// Summary converts a full LegalReview into its minimal API summary projection.
func (lr *LegalReview) Summary() *LegalReviewSummary {
	if lr == nil {
		return nil
	}
	return &LegalReviewSummary{
		ID:                        lr.ID,
		ReviewerID:                lr.ReviewerID,
		ExternalCounselRef:        lr.ExternalCounselRef,
		Decision:                  lr.Decision,
		ProhibitedChecklistPassed: lr.ProhibitedChecklistPassed,
		ReviewedAt:                lr.ReviewedAt,
	}
}
