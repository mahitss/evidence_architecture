package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestGeoblock(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	r.Group(func(reader chi.Router) {
		reader.Use(Geoblock)
		reader.Get("/api/v1/cases", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":["secret-published-case"]}`))
		})
	})
	// Admin route intentionally NOT wrapped with Geoblock
	r.Get("/api/v1/admin/cases/123", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	blockedCountries := []string{
		"AT", "BE", "BG", "HR", "CY", "CZ", "DK", "EE", "FI", "FR",
		"DE", "GR", "HU", "IE", "IT", "LV", "LT", "LU", "MT", "NL",
		"PL", "PT", "RO", "SK", "SI", "ES", "SE", "IS", "LI", "NO",
		"GB", "UK", "de", " fr ",
	}
	for _, cc := range blockedCountries {
		cc := cc
		t.Run("EU_EEA_UK_blocked_"+cc, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/cases", nil)
			req.Header.Set("CF-IPCountry", cc)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusUnavailableForLegalReasons, rec.Code)
			assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
			assert.Contains(t, rec.Body.String(), "Not available in your region")
			assert.NotContains(t, rec.Body.String(), "secret-published-case")
		})
	}

	t.Run("US country is allowed", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cases", nil)
		req.Header.Set("CF-IPCountry", "US")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "secret-published-case")
	})

	t.Run("CA and JP countries are allowed", func(t *testing.T) {
		t.Parallel()
		for _, cc := range []string{"CA", "JP", "AU"} {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/cases", nil)
			req.Header.Set("CF-IPCountry", cc)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})

	t.Run("missing CF-IPCountry header allowed in local dev", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cases", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "secret-published-case")
	})

	t.Run("admin routes bypass geoblock even for EU country header", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/cases/123", nil)
		req.Header.Set("CF-IPCountry", "DE")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"ok":true`)
	})
}

func TestGeoblockRunsBeforeRateLimit(t *testing.T) {
	t.Parallel()

	limiter := NewMemoryRateLimiter()
	r := chi.NewRouter()
	r.Use(RequestID)
	r.Group(func(reader chi.Router) {
		reader.Use(Geoblock)
		reader.Use(RateLimit(limiter, RateLimitConfig{
			Bucket: "test-order",
			Limit:  1,
			Window: 60,
		}))
		reader.Get("/api/v1/cases", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	// 5 blocked EU requests from the same IP must NOT consume the 1-request budget.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cases", nil)
		req.RemoteAddr = "198.51.100.10:12345"
		req.Header.Set("CF-IPCountry", "DE")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnavailableForLegalReasons, rec.Code)
	}

	// Subsequent US request from same IP still gets its full budget.
	reqUS := httptest.NewRequest(http.MethodGet, "/api/v1/cases", nil)
	reqUS.RemoteAddr = "198.51.100.10:12345"
	reqUS.Header.Set("CF-IPCountry", "US")
	recUS := httptest.NewRecorder()
	r.ServeHTTP(recUS, reqUS)
	assert.Equal(t, http.StatusOK, recUS.Code)
}
