package queue

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPublisher_RefusesNonLoopbackRedisURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	badURLs := []string{
		"redis://prod.redis.internal:6379/0",
		"redis://0.0.0.0:6379/0",
		"redis://localhost:6379/0",
	}

	for _, u := range badURLs {
		_, err := NewPublisher(ctx, u)
		require.Error(t, err, "REDIS_URL %q must be refused", u)
		assert.ErrorIs(t, err, ErrNonLoopbackRedis)
	}
}

func TestPublisher_PublishAndIdempotencyDedupe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pub, err := NewPublisher(ctx, DefaultRedisURL)
	require.NoError(t, err, "local Redis 7 must be running on 127.0.0.1:6379 (make services-up)")
	t.Cleanup(func() {
		_ = pub.Close()
	})

	// Use an isolated stream name for unit test cleanliness while verifying real XADD + SetNX
	testStream := "evidence.process.test." + uuid.NewString()
	pub.stream = testStream
	jobID := uuid.NewString()
	dedupeKey := "published:" + jobID

	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cleanCancel()
		_ = pub.Client().Del(cleanCtx, testStream, dedupeKey).Err()
	})

	evID := uuid.New()
	caseID := uuid.New()
	job := EvidenceProcessingJob{
		JobID:      jobID,
		EvidenceID: evID,
		StorageKey: "cases/" + caseID.String() + "/evidence/" + evID.String() + "/doc.pdf",
		MimeType:   "application/pdf",
		CaseID:     caseID,
	}

	// First publish adds one entry to the stream
	require.NoError(t, pub.PublishEvidenceJob(ctx, job))

	// Second publish with the same job_id is deduped cleanly
	require.NoError(t, pub.PublishEvidenceJob(ctx, job))

	entries, err := pub.Client().XRange(ctx, testStream, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1, "idempotency key must prevent duplicate stream entry")
	assert.Equal(t, jobID, entries[0].Values["job_id"])
	assert.Equal(t, evID.String(), entries[0].Values["evidence_id"])
	assert.Equal(t, caseID.String(), entries[0].Values["case_id"])
	assert.Equal(t, "application/pdf", entries[0].Values["mime_type"])

	ttl, err := pub.Client().TTL(ctx, dedupeKey).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, 23*time.Hour)
}
