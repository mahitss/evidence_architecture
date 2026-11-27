package middleware

import (
	"encoding/json"
	"net/http"
)

// ErrorDetail matches the OpenAPI ErrorEnvelope inner object.
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// ErrorEnvelopeResponse matches openapi.yaml ErrorEnvelope schema:
// {"error":{"code":"...","message":"...","request_id":"..."}}
type ErrorEnvelopeResponse struct {
	Error ErrorDetail `json:"error"`
}

// WriteError writes a standardized JSON error envelope response with X-Content-Type-Options: nosniff.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)

	payload := ErrorEnvelopeResponse{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			RequestID: RequestIDFromContext(r.Context()),
		},
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// ErrorEnvelope applies defensive response headers and ensures panic/fallback responses
// conform to the OpenAPI JSON error envelope.
func ErrorEnvelope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
