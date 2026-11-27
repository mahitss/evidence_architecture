package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const minJWTSecretLen = 32

var (
	// ErrInvalidToken indicates a malformed, unsigned, or tampered JWT.
	ErrInvalidToken = errors.New("invalid token")
	// ErrTokenExpired indicates a validly signed JWT whose exp claim is in the past.
	ErrTokenExpired = errors.New("token expired")
	// ErrInvalidSecret indicates the configured JWT HMAC secret is too short.
	ErrInvalidSecret = errors.New("jwt secret must be at least 32 characters")
)

// Claims represents validated admin JWT claims.
type Claims struct {
	Subject   string
	TokenID   string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// JWTManager signs and verifies HS256 JWTs without global state.
type JWTManager struct {
	secret []byte
	now    func() time.Time
}

// NewJWTManager constructs a JWTManager validated against minimum secret entropy length.
func NewJWTManager(secret string) (*JWTManager, error) {
	if len(secret) < minJWTSecretLen {
		return nil, ErrInvalidSecret
	}
	return &JWTManager{
		secret: []byte(secret),
		now:    time.Now,
	}, nil
}

// Issue creates a signed HS256 JWT containing sub, iat, exp, and jti claims.
func (m *JWTManager) Issue(subject string, ttl time.Duration) (string, error) {
	if subject == "" {
		return "", errors.New("jwt subject must not be empty")
	}
	if ttl <= 0 {
		return "", errors.New("jwt ttl must be positive")
	}

	now := m.now().UTC()
	regClaims := jwt.RegisteredClaims{
		Subject:   subject,
		ID:        uuid.NewString(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, regClaims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}

// Verify parses and validates an HS256 JWT and returns its structured Claims.
func (m *JWTManager) Verify(tokenString string) (Claims, error) {
	if tokenString == "" {
		return Claims{}, ErrInvalidToken
	}

	var regClaims jwt.RegisteredClaims
	parsed, err := jwt.ParseWithClaims(
		tokenString,
		&regClaims,
		func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return Claims{}, ErrTokenExpired
		}
		return Claims{}, ErrInvalidToken
	}
	if parsed == nil || !parsed.Valid {
		return Claims{}, ErrInvalidToken
	}
	if regClaims.Subject == "" || regClaims.ID == "" || regClaims.IssuedAt == nil || regClaims.ExpiresAt == nil {
		return Claims{}, ErrInvalidToken
	}

	return Claims{
		Subject:   regClaims.Subject,
		TokenID:   regClaims.ID,
		IssuedAt:  regClaims.IssuedAt.Time,
		ExpiresAt: regClaims.ExpiresAt.Time,
	}, nil
}
