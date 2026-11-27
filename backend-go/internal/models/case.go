package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Case represents a row in the cases table.
type Case struct {
	ID                      uuid.UUID       `json:"id"`
	Slug                    string          `json:"slug"`
	Title                   string          `json:"title"`
	Category                string          `json:"category"`
	Jurisdiction            string          `json:"jurisdiction"`
	EUOrigin                bool            `json:"eu_origin"`
	GDPRVerified            bool            `json:"gdpr_verified"`
	Status                  string          `json:"status"`
	OfficialBriefing        string          `json:"official_briefing"`
	GenuineGaps             json.RawMessage `json:"genuine_gaps"`
	PublishedCorrectionsLog json.RawMessage `json:"published_corrections_log"`
	PublishedLegalReviewID  *uuid.UUID      `json:"published_legal_review_id"`
	SearchSyncPending       bool            `json:"search_sync_pending"`
	PublishedAt             *time.Time      `json:"published_at"`
	RetractedAt             *time.Time      `json:"retracted_at"`
	RetractionReason        *string         `json:"retraction_reason"`
	CreatedBy               uuid.UUID       `json:"created_by"`
	UpdatedBy               uuid.UUID       `json:"updated_by"`
	CreatedAt               time.Time       `json:"created_at"`
	UpdatedAt               time.Time       `json:"updated_at"`
}
