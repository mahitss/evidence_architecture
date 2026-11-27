package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      uint32 = 64 * 1024 // 64 MB in KiB
	argonIterations  uint32 = 3
	argonParallelism uint8  = 2
	argonSaltLen     uint32 = 16
	argonKeyLen      uint32 = 32
)

var (
	// ErrPasswordMismatch indicates the plaintext password does not match the stored Argon2id hash.
	ErrPasswordMismatch = errors.New("password does not match")
	// ErrInvalidHashFormat indicates the stored hash is not a valid Argon2id PHC string.
	ErrInvalidHashFormat = errors.New("invalid argon2id hash format")
	// ErrEmptyPassword indicates an empty password was provided for hashing.
	ErrEmptyPassword = errors.New("password must not be empty")
)

// HashPassword hashes a plaintext password with Argon2id using OWASP-aligned parameters
// (64MB memory, 3 iterations, parallelism 2, 16-byte random salt, 32-byte derived key).
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", ErrEmptyPassword
	}

	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonIterations,
		argonParallelism,
		b64Salt,
		b64Hash,
	), nil
}

// VerifyPassword verifies a plaintext password against a PHC-formatted Argon2id hash
// using constant-time byte comparison.
func VerifyPassword(encodedHash, password string) error {
	params, salt, expectedKey, err := decodeArgon2idHash(encodedHash)
	if err != nil {
		return err
	}

	computedKey := argon2.IDKey(
		[]byte(password),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		params.keyLen,
	)

	if subtle.ConstantTimeCompare(computedKey, expectedKey) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}

type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	keyLen      uint32
}

func decodeArgon2idHash(encodedHash string) (argon2Params, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return argon2Params{}, nil, nil, ErrInvalidHashFormat
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return argon2Params{}, nil, nil, ErrInvalidHashFormat
	}

	var p argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return argon2Params{}, nil, nil, ErrInvalidHashFormat
	}
	if p.memory == 0 || p.iterations == 0 || p.parallelism == 0 {
		return argon2Params{}, nil, nil, ErrInvalidHashFormat
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return argon2Params{}, nil, nil, ErrInvalidHashFormat
	}

	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return argon2Params{}, nil, nil, ErrInvalidHashFormat
	}
	p.keyLen = uint32(len(key))

	return p, salt, key, nil
}
