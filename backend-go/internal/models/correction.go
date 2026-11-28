package models

import (
	"time"

	"github.com/google/uuid"
)

// CorrectionRequest represents a row in correction_requests.
type CorrectionRequest struct {
	ID                    uuid.UUID  `json:"id"`
	TicketID              string     `json:"ticket_id"`
	CaseID                *uuid.UUID `json:"case_id,omitempty"`
	EvidenceID            *uuid.UUID `json:"evidence_id,omitempty"`
	RequestType           string     `json:"request_type"`
	RequesterContactEmail string     `json:"requester_contact_email"`
	Description           string     `json:"description"`
	SupportingCitation    *string    `json:"supporting_citation,omitempty"`
	Status                string     `json:"status"`
	CreatedAt             time.Time  `json:"created_at"`
}
