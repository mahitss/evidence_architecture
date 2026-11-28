package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// IdempotencyKeyHeader is the required header on mutating admin/public write endpoints.
	IdempotencyKeyHeader = "Idempotency-Key"
	// IdempotencyKeyPrefix namespaces idempotency records in Redis.
	IdempotencyKeyPrefix = "idem:"
	// IdempotencyTTL is the retention window for cached idempotent responses.
	IdempotencyTTL = 24 * time.Hour

	maxIdempotentBodyBytes int64 = 27 << 20 // 27 MiB (accommodates 25 MB upload + multipart metadata)
)

var validIdempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

// ErrIdempotencyNotFound indicates no cached response exists for the given idempotency key.
var ErrIdempotencyNotFound = errors.New("idempotency key not found")

// IdempotencyResponse is the JSON value stored in Redis under idem:{idempotency_key}.
type IdempotencyResponse struct {
	StatusCode        int    `json:"status_code"`
	ContentType       string `json:"content_type,omitempty"`
	ResponseBody      string `json:"response_body"`
	RequestHashSHA256 string `json:"request_hash_sha256"`
}

// IdempotencyStore abstracts idempotency cache persistence.
type IdempotencyStore interface {
	Get(ctx context.Context, key string) (IdempotencyResponse, error)
	Set(ctx context.Context, key string, resp IdempotencyResponse, ttl time.Duration) error
}

// RedisIdempotencyStore persists idempotency records in Redis using go-redis.
type RedisIdempotencyStore struct {
	client *redis.Client
}

// NewRedisIdempotencyStore wraps a *redis.Client as an IdempotencyStore.
func NewRedisIdempotencyStore(client *redis.Client) *RedisIdempotencyStore {
	return &RedisIdempotencyStore{client: client}
}

// Get retrieves a cached response by idempotency key (stored under idem:{key}).
func (s *RedisIdempotencyStore) Get(ctx context.Context, key string) (IdempotencyResponse, error) {
	if s == nil || s.client == nil {
		return IdempotencyResponse{}, errors.New("redis idempotency store is not initialized")
	}
	raw, err := s.client.Get(ctx, IdempotencyKeyPrefix+key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return IdempotencyResponse{}, ErrIdempotencyNotFound
		}
		return IdempotencyResponse{}, fmt.Errorf("redis get idempotency key: %w", err)
	}
	var resp IdempotencyResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return IdempotencyResponse{}, fmt.Errorf("unmarshal idempotency record: %w", err)
	}
	return resp, nil
}

// Set stores a captured response under idem:{key} with the specified TTL.
func (s *RedisIdempotencyStore) Set(ctx context.Context, key string, resp IdempotencyResponse, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return errors.New("redis idempotency store is not initialized")
	}
	payload, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal idempotency record: %w", err)
	}
	if err := s.client.Set(ctx, IdempotencyKeyPrefix+key, payload, ttl).Err(); err != nil {
		return fmt.Errorf("redis set idempotency key: %w", err)
	}
	return nil
}

// Idempotency enforces the Idempotency-Key header on mutating HTTP methods (POST, PUT, PATCH, DELETE).
// It fails closed (HTTP 500) if the backing IdempotencyStore is unavailable.
func Idempotency(idemStore IdempotencyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isMutatingMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			key := strings.TrimSpace(r.Header.Get(IdempotencyKeyHeader))
			if key == "" {
				WriteError(w, r, http.StatusBadRequest, "MISSING_IDEMPOTENCY_KEY", "Idempotency-Key header is required")
				return
			}
			if !validIdempotencyKeyPattern.MatchString(key) {
				WriteError(w, r, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "Idempotency-Key must match ^[A-Za-z0-9_-]{16,64}$")
				return
			}

			if idemStore == nil {
				WriteError(w, r, http.StatusInternalServerError, "IDEMPOTENCY_STORE_ERROR", "idempotency store unavailable")
				return
			}

			// Read and hash request body (bounded) so we can detect body mismatches on replay
			// and restore r.Body transparently for downstream handlers.
			var bodyBytes []byte
			if r.Body != nil {
				limited := io.LimitReader(r.Body, maxIdempotentBodyBytes+1)
				var err error
				bodyBytes, err = io.ReadAll(limited)
				if err != nil {
					WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "failed to read request body")
					return
				}
				if int64(len(bodyBytes)) > maxIdempotentBodyBytes {
					WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body exceeds maximum allowed size")
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}

			sum := sha256.Sum256(bodyBytes)
			reqHash := hex.EncodeToString(sum[:])

			stored, err := idemStore.Get(r.Context(), key)
			if err == nil {
				if stored.RequestHashSHA256 != reqHash {
					WriteError(w, r, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Idempotency-Key reused with different body")
					return
				}
				ct := stored.ContentType
				if ct == "" {
					ct = "application/json; charset=utf-8"
				}
				w.Header().Set("Content-Type", ct)
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.Header().Set("X-Idempotent-Replayed", "true")
				status := stored.StatusCode
				if status == 0 {
					status = http.StatusOK
				}
				w.WriteHeader(status)
				_, _ = io.WriteString(w, stored.ResponseBody)
				return
			}
			if !errors.Is(err, ErrIdempotencyNotFound) {
				// Fail closed: idempotency is a safety control, never silently bypass when Redis is down.
				WriteError(w, r, http.StatusInternalServerError, "IDEMPOTENCY_STORE_ERROR", "idempotency store unavailable")
				return
			}

			capWriter := newCaptureResponseWriter()
			next.ServeHTTP(capWriter, r)

			statusCode := capWriter.statusCode
			if statusCode == 0 {
				statusCode = http.StatusOK
			}
			contentType := capWriter.header.Get("Content-Type")

			if err := idemStore.Set(r.Context(), key, IdempotencyResponse{
				StatusCode:        statusCode,
				ContentType:       contentType,
				ResponseBody:      capWriter.body.String(),
				RequestHashSHA256: reqHash,
			}, IdempotencyTTL); err != nil {
				WriteError(w, r, http.StatusInternalServerError, "IDEMPOTENCY_STORE_ERROR", "failed to persist idempotency response")
				return
			}

			for k, vals := range capWriter.header {
				for _, v := range vals {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(statusCode)
			_, _ = w.Write(capWriter.body.Bytes())
		})
	}
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

type captureResponseWriter struct {
	header     http.Header
	statusCode int
	body       bytes.Buffer
}

func newCaptureResponseWriter() *captureResponseWriter {
	return &captureResponseWriter{header: make(http.Header)}
}

func (c *captureResponseWriter) Header() http.Header {
	return c.header
}

func (c *captureResponseWriter) WriteHeader(code int) {
	if c.statusCode == 0 {
		c.statusCode = code
	}
}

func (c *captureResponseWriter) Write(p []byte) (int, error) {
	if c.statusCode == 0 {
		c.statusCode = http.StatusOK
	}
	return c.body.Write(p)
}
