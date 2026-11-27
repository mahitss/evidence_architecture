package storage

import (
	"context"
	"io"
	"time"
)

// Storage defines the S3-compatible object storage boundary per ADR-013.
// LocalStorage implements this interface for local development; an S3/R2/B2
// implementation can be swapped in without changing handlers or domain logic.
type Storage interface {
	Put(ctx context.Context, key string, r io.Reader, contentType string) (url string, err error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	URL(ctx context.Context, key string, ttl time.Duration) (string, error)
}
