package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
)

// AuditEntry describes a single immutable record written to the append-only audit_log table.
type AuditEntry struct {
	Action        string
	ActorID       *uuid.UUID
	ActorRole     string
	TargetType    string
	TargetID      *uuid.UUID
	ReasonCode    *string
	Metadata      map[string]any
	PayloadSHA256 *string
	RequestID     string
}

// AuditStore appends records to the PostgreSQL audit_log table inside caller transactions.
type AuditStore struct{}

// Append inserts an immutable audit_log row inside the caller's active transaction.
// Request ID is automatically extracted from ctx when not explicitly provided on entry.
func (a *AuditStore) Append(ctx context.Context, tx pgx.Tx, entry AuditEntry) error {
	if tx == nil {
		return fmt.Errorf("audit_log append requires an active transaction")
	}

	role := entry.ActorRole
	if role == "" {
		if entry.ActorID != nil {
			role = "ADMIN"
		} else {
			role = "SYSTEM"
		}
	}

	meta := make(map[string]any, len(entry.Metadata)+1)
	for k, v := range entry.Metadata {
		meta[k] = v
	}

	reqID := entry.RequestID
	if reqID == "" {
		reqID = middleware.RequestIDFromContext(ctx)
	}
	if reqID != "" {
		meta["request_id"] = reqID
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}

	const query = `
		INSERT INTO audit_log (
			actor_admin_id,
			actor_role,
			action,
			entity_type,
			entity_id,
			reason_code,
			metadata,
			payload_sha256
		) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
	`

	if _, err := tx.Exec(
		ctx,
		query,
		entry.ActorID,
		role,
		entry.Action,
		entry.TargetType,
		entry.TargetID,
		entry.ReasonCode,
		string(metaBytes),
		entry.PayloadSHA256,
	); err != nil {
		return fmt.Errorf("append audit_log (%s): %w", entry.Action, err)
	}

	return nil
}
