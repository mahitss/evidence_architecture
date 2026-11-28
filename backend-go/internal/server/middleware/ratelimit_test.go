package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type failingRateLimiter struct{}

func (failingRateLimiter) Allow(_ context.Context, _ string, _ int, _ time.Duration) (bool, int, error) {
	return false, 0, errors.New("redis connection refused")
}

func TestRateLimitEnforcesLimitAndRetryAfter(t *testing.T) {
	limiter := NewMemoryRateLimiter()
	handler := RateLimit(limiter, RateLimitConfig{
		Bucket: "search",
		Limit:  3,
		Window: 60,
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
		req.RemoteAddr = "203.0.113.10:41234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d expected 200, got %d", i, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	req.RemoteAddr = "203.0.113.10:41234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request expected 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected non-empty Retry-After header on 429 response")
	}

	// Separate IP must not be rate-limited.
	reqOther := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	reqOther.RemoteAddr = "203.0.113.11:51234"
	recOther := httptest.NewRecorder()
	handler.ServeHTTP(recOther, reqOther)
	if recOther.Code != http.StatusOK {
		t.Fatalf("distinct IP expected 200, got %d", recOther.Code)
	}
}

func TestRateLimitPreservesBudgetWhenPrecededByGeoblock(t *testing.T) {
	limiter := NewMemoryRateLimiter()
	rl := RateLimit(limiter, RateLimitConfig{
		Bucket: "cases",
		Limit:  1,
		Window: 60,
	})
	chain := Geoblock(rl(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	// EU visitor gets 451 repeatedly without consuming the single allowed token for that IP.
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cases", nil)
		req.RemoteAddr = "198.51.100.7:9999"
		req.Header.Set("CF-IPCountry", "DE")
		rec := httptest.NewRecorder()
		chain.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnavailableForLegalReasons {
			t.Fatalf("expected 451 for DE, got %d", rec.Code)
		}
	}

	// US request from same IP still has its rate-limit token intact.
	reqUS := httptest.NewRequest(http.MethodGet, "/api/v1/cases", nil)
	reqUS.RemoteAddr = "198.51.100.7:9999"
	reqUS.Header.Set("CF-IPCountry", "US")
	recUS := httptest.NewRecorder()
	chain.ServeHTTP(recUS, reqUS)
	if recUS.Code != http.StatusOK {
		t.Fatalf("expected 200 after DE requests did not consume rate limit, got %d", recUS.Code)
	}
}

func TestRateLimitFailClosedReturns503(t *testing.T) {
	handler := RateLimit(failingRateLimiter{}, RateLimitConfig{
		Bucket:     "corrections",
		Limit:      3,
		Window:     3600,
		FailClosed: true,
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corrections", nil)
	req.RemoteAddr = "198.51.100.9:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when FailClosed limiter errors, got %d", rec.Code)
	}
}
