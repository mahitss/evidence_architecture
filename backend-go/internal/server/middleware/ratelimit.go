package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimitConfig specifies a per-IP fixed-window rate limit for a route group.
type RateLimitConfig struct {
	Bucket     string
	Limit      int
	Window     int  // window duration in seconds
	FailClosed bool // if true, return 503 when the backing store fails (e.g., T4 corrections)
}

// RateLimiter checks and increments per-IP request counts within a fixed window.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, retryAfterSec int, err error)
}

// MemoryRateLimiter is a thread-safe in-memory per-IP sliding-window rate limiter.
type MemoryRateLimiter struct {
	mu      sync.Mutex
	entries map[string][]time.Time
	now     func() time.Time
}

// NewMemoryRateLimiter creates a new in-memory RateLimiter.
func NewMemoryRateLimiter() *MemoryRateLimiter {
	return &MemoryRateLimiter{
		entries: make(map[string][]time.Time),
		now:     time.Now,
	}
}

// Allow checks whether key is within limit for the trailing window duration.
func (m *MemoryRateLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	cutoff := now.Add(-window)
	prev := m.entries[key]
	kept := prev[:0]
	for _, ts := range prev {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}

	if len(kept) >= limit {
		m.entries[key] = kept
		oldest := kept[0]
		retryAfter := int(oldest.Add(window).Sub(now).Seconds()) + 1
		if retryAfter < 1 {
			retryAfter = 1
		}
		return false, retryAfter, nil
	}

	kept = append(kept, now)
	m.entries[key] = kept
	return true, 0, nil
}

// RedisRateLimiter implements a fixed-window per-IP rate limiter backed by Redis.
type RedisRateLimiter struct {
	client *redis.Client
}

// NewRedisRateLimiter constructs a Redis-backed RateLimiter.
func NewRedisRateLimiter(client *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{client: client}
}

// Allow increments the Redis counter for key and enforces limit within window.
func (rl *RedisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error) {
	if rl == nil || rl.client == nil {
		return false, 0, fmt.Errorf("redis rate limiter not initialized")
	}

	redisKey := "rl:" + key
	count, err := rl.client.Incr(ctx, redisKey).Result()
	if err != nil {
		return false, 0, fmt.Errorf("redis incr rate limit: %w", err)
	}
	if count == 1 {
		if err := rl.client.Expire(ctx, redisKey, window).Err(); err != nil {
			return false, 0, fmt.Errorf("redis expire rate limit: %w", err)
		}
	}
	if int(count) > limit {
		ttl, err := rl.client.TTL(ctx, redisKey).Result()
		retryAfter := int(window.Seconds())
		if err == nil && ttl > 0 {
			retryAfter = int(ttl.Seconds()) + 1
		}
		if retryAfter < 1 {
			retryAfter = 1
		}
		return false, retryAfter, nil
	}
	return true, 0, nil
}

// RateLimit returns middleware enforcing per-IP request limits for cfg.Bucket.
func RateLimit(limiter RateLimiter, cfg RateLimitConfig) func(http.Handler) http.Handler {
	window := time.Duration(cfg.Window) * time.Second
	if window <= 0 {
		window = time.Minute
	}
	if limiter == nil {
		limiter = NewMemoryRateLimiter()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIPFromRequest(r)
			key := cfg.Bucket + ":" + ip

			allowed, retryAfter, err := limiter.Allow(r.Context(), key, cfg.Limit, window)
			if err != nil {
				if cfg.FailClosed {
					WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "rate limiter unavailable")
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				WriteError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "rate limit exceeded; please retry later")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ClientIPFromRequest extracts the caller IP, preferring CF-Connecting-IP (set by Cloudflare edge)
// when it parses as a valid IP address, and falling back to RemoteAddr host.
func ClientIPFromRequest(r *http.Request) string {
	if cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
		if ip := net.ParseIP(cfIP); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return "unknown"
}
