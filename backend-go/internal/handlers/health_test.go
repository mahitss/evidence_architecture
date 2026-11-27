package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockHealthChecker struct {
	err error
}

func (m mockHealthChecker) Health(_ context.Context) error {
	return m.err
}

func TestHealthHandler_OK(t *testing.T) {
	t.Parallel()

	h := NewHealthHandler(mockHealthChecker{err: nil}, "0.1.0-test")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, "ok", body["db"])
	assert.Equal(t, "0.1.0-test", body["version"])
}

func TestHealthHandler_DBDegradedReturns503(t *testing.T) {
	t.Parallel()

	h := NewHealthHandler(mockHealthChecker{err: errors.New("connection refused")}, "0.1.0-test")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "degraded", body["status"])
	assert.Equal(t, "degraded", body["db"])
	assert.Equal(t, "0.1.0-test", body["version"])
}
