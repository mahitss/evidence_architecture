package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testJWTSecret = "test-jwt-secret-with-at-least-32-characters!"

func TestJWTManager_IssueAndVerify(t *testing.T) {
	t.Parallel()

	mgr, err := NewJWTManager(testJWTSecret)
	require.NoError(t, err)

	subject := "00000000-0000-0000-0000-000000000001"
	token, err := mgr.Issue(subject, time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := mgr.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, subject, claims.Subject)
	assert.NotEmpty(t, claims.TokenID)
	assert.False(t, claims.IssuedAt.IsZero())
	assert.True(t, claims.ExpiresAt.After(claims.IssuedAt))
}

func TestJWTManager_ExpiredTokenFails(t *testing.T) {
	t.Parallel()

	mgr, err := NewJWTManager(testJWTSecret)
	require.NoError(t, err)

	past := time.Now().UTC().Add(-2 * time.Hour)
	mgr.now = func() time.Time { return past }

	token, err := mgr.Issue("00000000-0000-0000-0000-000000000001", 30*time.Minute)
	require.NoError(t, err)

	// Restore real clock so token is 90 minutes expired
	mgr.now = time.Now

	_, err = mgr.Verify(token)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTokenExpired)
}

func TestJWTManager_TamperedTokenFails(t *testing.T) {
	t.Parallel()

	mgr, err := NewJWTManager(testJWTSecret)
	require.NoError(t, err)

	token, err := mgr.Issue("00000000-0000-0000-0000-000000000001", time.Hour)
	require.NoError(t, err)

	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)

	// Flip last character of signature
	sig := parts[2]
	flipped := "A"
	if sig[len(sig)-1] == 'A' {
		flipped = "B"
	}
	tampered := parts[0] + "." + parts[1] + "." + sig[:len(sig)-1] + flipped

	_, err = mgr.Verify(tampered)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestJWTManager_DifferentSecretFails(t *testing.T) {
	t.Parallel()

	mgr1, err := NewJWTManager(testJWTSecret)
	require.NoError(t, err)
	mgr2, err := NewJWTManager("another-completely-different-32-char-secret!")
	require.NoError(t, err)

	token, err := mgr1.Issue("00000000-0000-0000-0000-000000000001", time.Hour)
	require.NoError(t, err)

	_, err = mgr2.Verify(token)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestNewJWTManager_ShortSecretRejected(t *testing.T) {
	t.Parallel()

	_, err := NewJWTManager("too-short")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSecret)
}
