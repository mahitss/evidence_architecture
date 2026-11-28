package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TimelineEvent represents a chronological row in case_timeline_events.
type TimelineEvent struct {
	ID                    uuid.UUID       `json:"id"`
	CaseID                uuid.UUID       `json:"case_id"`
	EventStartDate        string          `json:"event_start_date"`
	EventEndDate          *string         `json:"event_end_date"`
	SortOrder             int             `json:"sort_order"`
	EventType             string          `json:"event_type"`
	Title                 string          `json:"title"`
	Summary               string          `json:"summary"`
	VerificationTier      int16           `json:"verification_tier"`
	LinkedEvidenceIDs     json.RawMessage `json:"linked_evidence_ids"`
	HasConflict           bool            `json:"has_conflict"`
	ConflictingSources    json.RawMessage `json:"conflicting_sources"`
	ControllingStatusNote *string         `json:"controlling_status_note"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}
