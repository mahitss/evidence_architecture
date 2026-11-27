package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotFound is returned when a requested database entity does not exist.
	ErrNotFound = errors.New("record not found")
	// ErrConflict indicates a uniqueness violation (e.g., duplicate slug) or disallowed state transition.
	ErrConflict = errors.New("conflict")
	// ErrUnprocessable indicates a PostgreSQL CHECK constraint or trigger domain rejection.
	ErrUnprocessable = errors.New("unprocessable entity")
)

// Store wraps a pgxpool.Pool and exposes domain repositories.
type Store struct {
	pool         *pgxpool.Pool
	Cases        *CaseStore
	AdminUsers   *AdminUserStore
	LegalReviews *LegalReviewStore
	Evidence     *EvidenceStore
	Audit        *AuditStore
}

// New opens a PostgreSQL connection pool, verifies connectivity with Ping, and returns Store.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	poolCfg.MaxConns = 20
	poolCfg.MinConns = 2
	poolCfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	audit := &AuditStore{}
	return &Store{
		pool:         pool,
		Cases:        &CaseStore{pool: pool, audit: audit},
		AdminUsers:   &AdminUserStore{pool: pool},
		LegalReviews: &LegalReviewStore{pool: pool, audit: audit},
		Evidence:     &EvidenceStore{pool: pool, audit: audit},
		Audit:        audit,
	}, nil
}

// Close releases all pooled PostgreSQL connections.
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// Health pings PostgreSQL and returns an error if the database is unreachable.
func (s *Store) Health(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return errors.New("database pool is not initialized")
	}
	if err := s.pool.Ping(ctx); err != nil {
		return fmt.Errorf("database health ping failed: %w", err)
	}
	return nil
}

// Begin starts a new PostgreSQL transaction on the underlying pool.
func (s *Store) Begin(ctx context.Context) (pgx.Tx, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("database pool is not initialized")
	}
	return s.pool.Begin(ctx)
}

// Pool exposes the underlying pgxpool.Pool for integration tests or transactions.
func (s *Store) Pool() *pgxpool.Pool {
	if s == nil {
		return nil
	}
	return s.pool
}

// PGErrorMessage returns the primary PostgreSQL server error message when err wraps a *pgconn.PgError.
func PGErrorMessage(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Message, true
	}
	return "", false
}

// IsPGUniqueViolation checks whether err is PostgreSQL SQLSTATE 23505.
func IsPGUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// IsPGConstraintOrTriggerError checks whether err is a CHECK (23514), FK (23503), NOT NULL (23502),
// or PL/pgSQL RAISE EXCEPTION (P0001).
func IsPGConstraintOrTriggerError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "P0001", "23514", "23503", "23502":
		return true
	default:
		return false
	}
}
