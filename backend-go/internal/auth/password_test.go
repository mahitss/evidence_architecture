package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndVerifyPassword_RoundTrip(t *testing.T) {
	t.Parallel()

	password := "CorrectHorseBatteryStaple!2026"
	hash1, err := HashPassword(password)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(hash1, "$argon2id$v=19$m=65536,t=3,p=2$"))

	hash2, err := HashPassword(password)
	require.NoError(t, err)
	assert.NotEqual(t, hash1, hash2, "each hash must use a unique random salt")

	require.NoError(t, VerifyPassword(hash1, password))
	require.NoError(t, VerifyPassword(hash2, password))
}

func TestVerifyPassword_WrongPasswordFails(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("SuperSecretAdminPass!99")
	require.NoError(t, err)

	err = VerifyPassword(hash, "WrongPassword!99")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPasswordMismatch)
}

func TestHashPassword_EmptyRejected(t *testing.T) {
	t.Parallel()

	_, err := HashPassword("")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEmptyPassword)
}

func TestVerifyPassword_InvalidOrDevSeedPlaceholderRejected(t *testing.T) {
	t.Parallel()

	invalidHashes := []string{
		"",
		"plaintext",
		"$bcrypt$v=19$m=65536,t=3,p=2$abc$def",
		"$argon2id$v=19$m=65536,t=3,p=1$DEVSEEDINVALIDNONAUTH$DEVSEEDCANNOTAUTHENTICATEINPRODUCTION",
	}

	for _, h := range invalidHashes {
		err := VerifyPassword(h, "anything")
		require.Error(t, err, "hash %q must not verify", h)
	}
}
