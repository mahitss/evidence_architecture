package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HealthChecker verifies backing datastore reachability.
type HealthChecker interface {
	Health(ctx context.Context) error
}

// HealthHandler serves GET /health without authentication.
type HealthHandler struct {
	checker HealthChecker
	version string
}

// NewHealthHandler creates a HealthHandler backed by the given HealthChecker.
func NewHealthHandler(checker HealthChecker, version string) *HealthHandler {
	return &HealthHandler{
		checker: checker,
		version: version,
	}
}

type healthResponse struct {
	Status  string `json:"status"`
	DB      string `json:"db"`
	Version string `json:"version"`
}

// ServeHTTP responds with 200 {"status":"ok","db":"ok","version":"..."} when healthy,
// or 503 {"status":"degraded","db":"degraded","version":"..."} if the DB ping fails.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if h.checker == nil || h.checker.Health(ctx) != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(healthResponse{
			Status:  "degraded",
			DB:      "degraded",
			Version: h.version,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(healthResponse{
		Status:  "ok",
		DB:      "ok",
		Version: h.version,
	})
}
