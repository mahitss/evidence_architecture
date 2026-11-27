package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a requested database entity does not exist.
var ErrNotFound = errors.New("record not found")

// Store wraps a pgxpool.Pool and exposes domain repositories.
type Store struct {
	pool         *pgxpool.Pool
	Cases        *CaseStore
	AdminUsers   *AdminUserStore
	LegalReviews *LegalReviewStore
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

	return &Store{
		pool:         pool,
		Cases:        &CaseStore{pool: pool},
		AdminUsers:   &AdminUserStore{pool: pool},
		LegalReviews: &LegalReviewStore{pool: pool},
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

// Pool exposes the underlying pgxpool.Pool for integration tests or transactions.
func (s *Store) Pool() *pgxpool.Pool {
	if s == nil {
		return nil
	}
	return s.pool
}
