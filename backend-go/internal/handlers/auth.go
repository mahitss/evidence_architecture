package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mahitss/evidence_architecture/backend-go/internal/auth"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
	"github.com/rs/zerolog"
)

const (
	defaultLoginFailureDelay = 150 * time.Millisecond
	loginRateWindow          = time.Minute
	maxLoginBodyBytes        = 8192
)

// AdminUserReader fetches admin user records for authentication.
type AdminUserReader interface {
	GetByEmail(ctx context.Context, email string) (*models.AdminUser, error)
}

// TokenIssuer mints signed admin JWTs.
type TokenIssuer interface {
	Issue(subject string, ttl time.Duration) (string, error)
}

type ipRateLimiter struct {
	mu          sync.Mutex
	maxAttempts int
	window      time.Duration
	now         func() time.Time
	attempts    map[string][]time.Time
}

func newIPRateLimiter(maxAttempts int, window time.Duration, nowFn func() time.Time) *ipRateLimiter {
	if maxAttempts < 1 {
		maxAttempts = 5
	}
	if nowFn == nil {
		nowFn = time.Now
	}
	return &ipRateLimiter{
		maxAttempts: maxAttempts,
		window:      window,
		now:         nowFn,
		attempts:    make(map[string][]time.Time),
	}
}

func (rl *ipRateLimiter) allow(ip string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()
	cutoff := now.Add(-rl.window)

	existing := rl.attempts[ip]
	pruned := existing[:0]
	for _, ts := range existing {
		if ts.After(cutoff) {
			pruned = append(pruned, ts)
		}
	}

	if len(pruned) >= rl.maxAttempts {
		rl.attempts[ip] = pruned
		retryAfter := pruned[0].Add(rl.window).Sub(now)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}

	pruned = append(pruned, now)
	rl.attempts[ip] = pruned
	return true, 0
}

// AuthHandler serves POST /api/v1/admin/login with in-memory per-IP rate limiting
// and constant-time Argon2id credential verification.
type AuthHandler struct {
	users        AdminUserReader
	tokens       TokenIssuer
	jwtTTL       time.Duration
	logger       zerolog.Logger
	limiter      *ipRateLimiter
	dummyHash    string
	failureDelay time.Duration
	now          func() time.Time
}

// NewAuthHandler creates a configured AuthHandler.
func NewAuthHandler(
	users AdminUserReader,
	tokens TokenIssuer,
	jwtTTL time.Duration,
	rateLimitPerMin int,
	logger zerolog.Logger,
) *AuthHandler {
	dummyHash, err := auth.HashPassword("dummy-timing-equalizer-password-never-used")
	if err != nil {
		// Fallback valid PHC string with 16-byte salt & 32-byte key if CSPRNG fails
		dummyHash = "$argon2id$v=19$m=65536,t=3,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	return &AuthHandler{
		users:        users,
		tokens:       tokens,
		jwtTTL:       jwtTTL,
		logger:       logger,
		limiter:      newIPRateLimiter(rateLimitPerMin, loginRateWindow, time.Now),
		dummyHash:    dummyHash,
		failureDelay: defaultLoginFailureDelay,
		now:          time.Now,
	}
}

// SetFailureDelay allows tests to adjust the anti-enumeration sleep duration.
func (h *AuthHandler) SetFailureDelay(d time.Duration) {
	h.failureDelay = d
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// Login handles POST /api/v1/admin/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	clientIP := extractClientIP(r)
	if allowed, retryAfter := h.limiter.allow(clientIP); !allowed {
		seconds := int(retryAfter.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		middleware.WriteError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "too many login attempts")
		return
	}

	var req loginRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, maxLoginBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON request body")
		return
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(req.Email))
	if normalizedEmail == "" || req.Password == "" {
		middleware.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "email and password are required")
		return
	}

	emailHash := hashEmailForLog(normalizedEmail)
	reqID := middleware.RequestIDFromContext(r.Context())

	user, err := h.users.GetByEmail(r.Context(), normalizedEmail)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			h.logger.Error().
				Err(err).
				Str("request_id", reqID).
				Str("email_hash", emailHash).
				Bool("success", false).
				Msg("admin_login_store_error")
			middleware.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "authentication service temporarily unavailable")
			return
		}
		// Equalize Argon2id work when email does not exist to prevent timing enumeration.
		_ = auth.VerifyPassword(h.dummyHash, req.Password)
		h.failAuth(w, r, emailHash, reqID)
		return
	}

	if !user.IsActive {
		_ = auth.VerifyPassword(h.dummyHash, req.Password)
		h.failAuth(w, r, emailHash, reqID)
		return
	}

	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		if errors.Is(err, auth.ErrInvalidHashFormat) {
			// Ensure non-authenticating dev-seed placeholders also execute full Argon2id work.
			_ = auth.VerifyPassword(h.dummyHash, req.Password)
		}
		h.failAuth(w, r, emailHash, reqID)
		return
	}

	expiresAt := h.now().UTC().Add(h.jwtTTL)
	token, err := h.tokens.Issue(user.ID.String(), h.jwtTTL)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("request_id", reqID).
			Str("email_hash", emailHash).
			Bool("success", false).
			Msg("admin_login_token_issue_failed")
		middleware.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to issue token")
		return
	}

	h.logger.Info().
		Str("request_id", reqID).
		Str("email_hash", emailHash).
		Bool("success", true).
		Msg("admin_login_attempt")

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(loginResponse{
		Token:     token,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	})
}

func (h *AuthHandler) failAuth(w http.ResponseWriter, r *http.Request, emailHash, reqID string) {
	if h.failureDelay > 0 {
		timer := time.NewTimer(h.failureDelay)
		select {
		case <-timer.C:
		case <-r.Context().Done():
			timer.Stop()
		}
	}

	h.logger.Warn().
		Str("request_id", reqID).
		Str("email_hash", emailHash).
		Bool("success", false).
		Msg("admin_login_attempt")

	middleware.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "invalid credentials")
}

func hashEmailForLog(normalizedEmail string) string {
	sum := sha256.Sum256([]byte(normalizedEmail))
	return hex.EncodeToString(sum[:])
}

func extractClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
