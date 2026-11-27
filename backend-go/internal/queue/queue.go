package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// EvidenceProcessStream is the Redis Stream key for evidence extraction jobs.
	EvidenceProcessStream = "evidence.process"
	// DefaultRedisURL targets the local sandbox Redis 7 server on 127.0.0.1.
	DefaultRedisURL = "redis://127.0.0.1:6379/0"
	// IdempotencyKeyTTL is the deduplication window for published:{job_id} keys.
	IdempotencyKeyTTL = 24 * time.Hour
	// MaxStreamApproxLen caps stream memory usage in local/VPS deployments.
	MaxStreamApproxLen int64 = 10000
)

// ErrNonLoopbackRedis is returned when REDIS_URL points outside 127.0.0.1.
var ErrNonLoopbackRedis = errors.New("refusing to connect to non-loopback REDIS_URL host (must be 127.0.0.1)")

// EvidenceProcessingJob is the payload published to Redis Stream 'evidence.process'.
type EvidenceProcessingJob struct {
	JobID      string    `json:"job_id"`
	EvidenceID uuid.UUID `json:"evidence_id"`
	StorageKey string    `json:"storage_key"`
	MimeType   string    `json:"mime_type"`
	CaseID     uuid.UUID `json:"case_id"`
}

// EvidencePublisher defines the queue boundary used by evidence upload handlers.
type EvidencePublisher interface {
	PublishEvidenceJob(ctx context.Context, job EvidenceProcessingJob) error
}

// Publisher wraps a Redis client to publish idempotent Redis Streams jobs.
type Publisher struct {
	client *redis.Client
	stream string
}

// NewPublisher parses redisURL (defaults to REDIS_URL env var or redis://127.0.0.1:6379/0),
// enforces the strict 127.0.0.1 loopback guard, pings Redis, and returns a Publisher.
func NewPublisher(ctx context.Context, redisURL string) (*Publisher, error) {
	if strings.TrimSpace(redisURL) == "" {
		redisURL = strings.TrimSpace(os.Getenv("REDIS_URL"))
	}
	if redisURL == "" {
		redisURL = DefaultRedisURL
	}

	if err := validateLoopbackRedisURL(redisURL); err != nil {
		return nil, err
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}

	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Publisher{
		client: client,
		stream: EvidenceProcessStream,
	}, nil
}

// Close closes the underlying Redis client connection pool.
func (p *Publisher) Close() error {
	if p == nil || p.client == nil {
		return nil
	}
	return p.client.Close()
}

// Client returns the underlying *redis.Client for integration/unit test assertions.
func (p *Publisher) Client() *redis.Client {
	if p == nil {
		return nil
	}
	return p.client
}

// PublishEvidenceJob publishes job to Redis Stream 'evidence.process' with 24-hour idempotency
// keyed on "published:{job_id}" so duplicate retries do not enqueue duplicate stream entries.
func (p *Publisher) PublishEvidenceJob(ctx context.Context, job EvidenceProcessingJob) error {
	if p == nil || p.client == nil {
		return errors.New("redis publisher is not initialized")
	}

	if strings.TrimSpace(job.JobID) == "" {
		job.JobID = uuid.NewString()
	}

	dedupeKey := "published:" + job.JobID
	claimed, err := p.client.SetNX(ctx, dedupeKey, "1", IdempotencyKeyTTL).Result()
	if err != nil {
		return fmt.Errorf("set idempotency key %s: %w", dedupeKey, err)
	}
	if !claimed {
		// Already published within the 24h idempotency window; safely dedupe retry.
		return nil
	}

	payload, err := json.Marshal(job)
	if err != nil {
		_ = p.client.Del(ctx, dedupeKey).Err()
		return fmt.Errorf("marshal evidence job: %w", err)
	}

	if err := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: p.stream,
		MaxLen: MaxStreamApproxLen,
		Approx: true,
		Values: map[string]any{
			"job_id":      job.JobID,
			"evidence_id": job.EvidenceID.String(),
			"storage_key": job.StorageKey,
			"mime_type":   job.MimeType,
			"case_id":     job.CaseID.String(),
			"payload":     string(payload),
		},
	}).Err(); err != nil {
		// Release idempotency claim so a legitimate retry can succeed once Redis recovers.
		_ = p.client.Del(ctx, dedupeKey).Err()
		return fmt.Errorf("xadd %s: %w", p.stream, err)
	}

	return nil
}

func validateLoopbackRedisURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid REDIS_URL: %w", err)
	}
	if parsed.Scheme != "redis" && parsed.Scheme != "rediss" {
		return fmt.Errorf("invalid REDIS_URL scheme %q", parsed.Scheme)
	}
	if parsed.Hostname() != "127.0.0.1" {
		return fmt.Errorf("%w: got %q", ErrNonLoopbackRedis, parsed.Hostname())
	}
	return nil
}
