package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memIdempotencyStore struct {
	mu      sync.Mutex
	records map[string]IdempotencyResponse
	ttls    map[string]time.Duration
	getErr  error
	setErr  error
}

func newMemIdempotencyStore() *memIdempotencyStore {
	return &memIdempotencyStore{
		records: make(map[string]IdempotencyResponse),
		ttls:    make(map[string]time.Duration),
	}
}

func (m *memIdempotencyStore) Get(_ context.Context, key string) (IdempotencyResponse, error) {
	if m.getErr != nil {
		return IdempotencyResponse{}, m.getErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[key]
	if !ok {
		return IdempotencyResponse{}, ErrIdempotencyNotFound
	}
	return rec, nil
}

func (m *memIdempotencyStore) Set(_ context.Context, key string, resp IdempotencyResponse, ttl time.Duration) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[key] = resp
	m.ttls[key] = ttl
	return nil
}

func TestIdempotencyMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("missing Idempotency-Key header returns 400 MISSING_IDEMPOTENCY_KEY", func(t *testing.T) {
		t.Parallel()
		st := newMemIdempotencyStore()
		var calls atomic.Int32
		h := RequestID(Idempotency(st)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusCreated)
		})))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases", strings.NewReader(`{"slug":"a"}`))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, int32(0), calls.Load())
		var env ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		assert.Equal(t, "MISSING_IDEMPOTENCY_KEY", env.Error.Code)
	})

	t.Run("invalid Idempotency-Key format returns 400 INVALID_IDEMPOTENCY_KEY", func(t *testing.T) {
		t.Parallel()
		st := newMemIdempotencyStore()
		var calls atomic.Int32
		h := RequestID(Idempotency(st)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusCreated)
		})))

		invalidKeys := []string{
			"too-short",
			strings.Repeat("a", 65),
			"contains spaces here!!123456",
			"bad/slash/character_123456",
		}
		for _, k := range invalidKeys {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases", strings.NewReader(`{}`))
			req.Header.Set(IdempotencyKeyHeader, k)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code, "key=%q", k)
			var env ErrorEnvelopeResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
			assert.Equal(t, "INVALID_IDEMPOTENCY_KEY", env.Error.Code)
		}
		assert.Equal(t, int32(0), calls.Load())
	})

	t.Run("first request passes through, second same body replays without calling handler, different body returns 409", func(t *testing.T) {
		t.Parallel()
		st := newMemIdempotencyStore()
		var calls atomic.Int32
		h := RequestID(Idempotency(st)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			n := calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"call": n, "status": "PENDING"})
		})))

		key := "idem_key_test_1234567890"
		bodyA := `{"title":"Case A"}`

		// First request
		req1 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases", strings.NewReader(bodyA))
		req1.Header.Set(IdempotencyKeyHeader, key)
		rec1 := httptest.NewRecorder()
		h.ServeHTTP(rec1, req1)

		require.Equal(t, http.StatusAccepted, rec1.Code)
		assert.Equal(t, int32(1), calls.Load())
		assert.Equal(t, 24*time.Hour, st.ttls[key])

		// Second request with identical body -> cached replay, handler NOT called
		req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases", strings.NewReader(bodyA))
		req2.Header.Set(IdempotencyKeyHeader, key)
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)

		require.Equal(t, http.StatusAccepted, rec2.Code)
		assert.Equal(t, int32(1), calls.Load(), "handler must NOT be called on idempotent replay")
		assert.Equal(t, rec1.Body.String(), rec2.Body.String())
		assert.Equal(t, "true", rec2.Header().Get("X-Idempotent-Replayed"))

		// Third request with same key but DIFFERENT body -> 409 IDEMPOTENCY_CONFLICT
		req3 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases", strings.NewReader(`{"title":"Case B"}`))
		req3.Header.Set(IdempotencyKeyHeader, key)
		rec3 := httptest.NewRecorder()
		h.ServeHTTP(rec3, req3)

		require.Equal(t, http.StatusConflict, rec3.Code)
		assert.Equal(t, int32(1), calls.Load())
		var env ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec3.Body.Bytes(), &env))
		assert.Equal(t, "IDEMPOTENCY_CONFLICT", env.Error.Code)
		assert.Equal(t, "Idempotency-Key reused with different body", env.Error.Message)
	})

	t.Run("GET and HEAD requests skip idempotency middleware", func(t *testing.T) {
		t.Parallel()
		st := newMemIdempotencyStore()
		st.getErr = errors.New("should never be queried for GET")
		var calls atomic.Int32
		h := RequestID(Idempotency(st)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusOK)
		})))

		for _, method := range []string{http.MethodGet, http.MethodHead} {
			req := httptest.NewRequest(method, "/api/v1/admin/cases/123", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)
		}
		assert.Equal(t, int32(2), calls.Load())
	})

	t.Run("Redis down fails closed with 500 IDEMPOTENCY_STORE_ERROR and does not call handler", func(t *testing.T) {
		t.Parallel()
		st := newMemIdempotencyStore()
		st.getErr = errors.New("dial tcp 127.0.0.1:6379: connection refused")
		var calls atomic.Int32
		h := RequestID(Idempotency(st)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusCreated)
		})))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases", strings.NewReader(`{"slug":"x"}`))
		req.Header.Set(IdempotencyKeyHeader, "valid_idempotency_key_0001")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Equal(t, int32(0), calls.Load(), "must fail closed when Redis is unavailable")
		var env ErrorEnvelopeResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
		assert.Equal(t, "IDEMPOTENCY_STORE_ERROR", env.Error.Code)
	})
}

func TestRedisIdempotencyStore_TTLIs24Hours(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://127.0.0.1:6379/0"
	}

	opts, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, client.Ping(ctx).Err(), "local Redis must be running on 127.0.0.1:6379")

	redisStore := NewRedisIdempotencyStore(client)
	key := "ttl_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	t.Cleanup(func() {
		_ = client.Del(context.Background(), IdempotencyKeyPrefix+key).Err()
	})

	h := RequestID(Idempotency(redisStore)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/cases", strings.NewReader(`{"slug":"ttl-check"}`))
	req.Header.Set(IdempotencyKeyHeader, key)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	ttl, err := client.TTL(ctx, IdempotencyKeyPrefix+key).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, 23*time.Hour+55*time.Minute, "Redis key TTL must be ~24h")
	assert.LessOrEqual(t, ttl, 24*time.Hour)
}
