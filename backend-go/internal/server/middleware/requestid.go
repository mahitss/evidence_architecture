package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type requestIDCtxKey struct{}

// HeaderRequestID is the canonical HTTP header carrying the request UUID.
const HeaderRequestID = "X-Request-ID"

// RequestID generates a UUID for every incoming request, stores it in the request
// context, and sets the X-Request-ID response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := uuid.NewString()
		w.Header().Set(HeaderRequestID, reqID)
		ctx := context.WithValue(r.Context(), requestIDCtxKey{}, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext extracts the request UUID injected by RequestID middleware.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(requestIDCtxKey{}).(string); ok {
		return v
	}
	return ""
}
