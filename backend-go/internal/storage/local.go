package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultLocalStorageRoot is the default filesystem root for LocalStorage.
	DefaultLocalStorageRoot = "./data/evidence"
	// MaxLocalUploadBytes enforces a 50 MB hard streaming cap on LocalStorage.Put.
	MaxLocalUploadBytes int64 = 50 * 1024 * 1024
)

var (
	// ErrInvalidKey indicates a storage key contains disallowed characters or path traversal.
	ErrInvalidKey = errors.New("invalid storage key")
	// ErrUnsupportedContentType indicates the MIME type is not in the storage allowlist.
	ErrUnsupportedContentType = errors.New("unsupported content type")
	// ErrFileTooLarge indicates the stream exceeded the configured byte size cap.
	ErrFileTooLarge = errors.New("file exceeds maximum allowed size")
	// ErrNotFound indicates the requested object does not exist in local storage.
	ErrNotFound = errors.New("storage object not found")
)

var allowedContentTypes = map[string]struct{}{
	"application/pdf":  {},
	"image/jpeg":       {},
	"image/png":        {},
	"image/webp":       {},
	"text/plain":       {},
	"application/json": {},
}

// Strict segment allowlist: alphanumeric, dash, underscore, dot (for extensions), separated by '/'.
// Path traversal ('..') and leading/trailing slashes are explicitly rejected in sanitizeKey.
var validKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

// LocalStorage writes evidence blobs under {root}/{key} for local development (ADR-013).
// Production deployments will swap this implementation for an S3-compatible client.
type LocalStorage struct {
	root       string
	maxBytes   int64
	signingKey []byte
}

// NewLocalStorage creates a LocalStorage rooted at root (defaults to LOCAL_STORAGE_ROOT
// env var or ./data/evidence).
func NewLocalStorage(root string) (*LocalStorage, error) {
	if strings.TrimSpace(root) == "" {
		root = strings.TrimSpace(os.Getenv("LOCAL_STORAGE_ROOT"))
	}
	if root == "" {
		root = DefaultLocalStorageRoot
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve local storage root: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o750); err != nil {
		return nil, fmt.Errorf("create local storage root %q: %w", absRoot, err)
	}

	return &LocalStorage{
		root:       absRoot,
		maxBytes:   MaxLocalUploadBytes,
		signingKey: []byte("local-dev-file-url-signer"),
	}, nil
}

// SetMaxBytes overrides the maximum upload size (primarily for unit tests).
func (ls *LocalStorage) SetMaxBytes(n int64) {
	if n > 0 {
		ls.maxBytes = n
	}
}

// IsAllowedContentType checks whether contentType is permitted by LocalStorage.
func IsAllowedContentType(contentType string) bool {
	ct := normalizeContentType(contentType)
	_, ok := allowedContentTypes[ct]
	return ok
}

func normalizeContentType(contentType string) string {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if idx := strings.IndexByte(ct, ';'); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	return ct
}

// sanitizeKey validates key characters and ensures the resolved path remains strictly
// confined inside ls.root (blocks ../ traversal, null bytes, backslashes, and escapes).
func (ls *LocalStorage) sanitizeKey(key string) (string, string, error) {
	if key == "" || strings.Contains(key, "\x00") || strings.Contains(key, `\`) {
		return "", "", fmt.Errorf("%w: empty or illegal characters", ErrInvalidKey)
	}
	if strings.HasPrefix(key, "/") || strings.HasSuffix(key, "/") || strings.Contains(key, "//") {
		return "", "", fmt.Errorf("%w: leading, trailing, or duplicate slash", ErrInvalidKey)
	}
	if !validKeyPattern.MatchString(key) {
		return "", "", fmt.Errorf("%w: contains disallowed characters", ErrInvalidKey)
	}

	segments := strings.Split(key, "/")
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." || strings.Contains(seg, "..") {
			return "", "", fmt.Errorf("%w: path traversal rejected", ErrInvalidKey)
		}
	}

	cleanRel := filepath.Clean(filepath.FromSlash(key))
	if filepath.IsAbs(cleanRel) || strings.HasPrefix(cleanRel, "..") {
		return "", "", fmt.Errorf("%w: escapes storage root", ErrInvalidKey)
	}

	fullPath := filepath.Join(ls.root, cleanRel)
	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return "", "", fmt.Errorf("%w: resolve path: %v", ErrInvalidKey, err)
	}

	rootPrefix := ls.root + string(os.PathSeparator)
	if !strings.HasPrefix(absFull, rootPrefix) {
		return "", "", fmt.Errorf("%w: resolved path outside storage root", ErrInvalidKey)
	}

	return filepath.ToSlash(cleanRel), absFull, nil
}

// Put streams content into {LOCAL_STORAGE_ROOT}/{key}, enforcing the MIME allowlist,
// strict key confinement, and a streaming byte-size cap (default 50 MB).
func (ls *LocalStorage) Put(ctx context.Context, key string, r io.Reader, contentType string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if r == nil {
		return "", errors.New("reader must not be nil")
	}
	if !IsAllowedContentType(contentType) {
		return "", fmt.Errorf("%w: %q", ErrUnsupportedContentType, contentType)
	}

	cleanKey, absPath, err := ls.sanitizeKey(key)
	if err != nil {
		return "", err
	}

	parentDir := filepath.Dir(absPath)
	if err := os.MkdirAll(parentDir, 0o750); err != nil {
		return "", fmt.Errorf("create parent directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(parentDir, ".upload-*")
	if err != nil {
		return "", fmt.Errorf("create temp upload file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	// Read at most maxBytes + 1 to detect overflow without buffering unbounded input in memory.
	limited := io.LimitReader(r, ls.maxBytes+1)
	written, err := io.Copy(tmpFile, limited)
	if err != nil {
		return "", fmt.Errorf("write upload stream: %w", err)
	}
	if written > ls.maxBytes {
		return "", fmt.Errorf("%w: limit is %d bytes", ErrFileTooLarge, ls.maxBytes)
	}
	if err := tmpFile.Sync(); err != nil {
		return "", fmt.Errorf("sync upload file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("close upload file: %w", err)
	}

	if err := os.Rename(tmpPath, absPath); err != nil {
		return "", fmt.Errorf("commit upload file: %w", err)
	}
	cleanupTmp = false

	return ls.URL(ctx, cleanKey, 5*time.Minute)
}

// Get opens the stored object at key for reading.
func (ls *LocalStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_, absPath, err := ls.sanitizeKey(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return nil, fmt.Errorf("open stored object: %w", err)
	}
	return f, nil
}

// Delete removes the stored object at key (idempotent if already absent).
func (ls *LocalStorage) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, absPath, err := ls.sanitizeKey(key)
	if err != nil {
		return err
	}
	if err := os.Remove(absPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete stored object: %w", err)
	}
	return nil
}

// URL returns a local-only signed file:// URL containing expiration and HMAC query parameters.
// Production S3-compatible implementations will return a pre-signed HTTPS URL.
func (ls *LocalStorage) URL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	cleanKey, absPath, err := ls.sanitizeKey(key)
	if err != nil {
		return "", err
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	expires := time.Now().UTC().Add(ttl).Unix()
	mac := hmac.New(sha256.New, ls.signingKey)
	mac.Write([]byte(cleanKey + ":" + strconv.FormatInt(expires, 10)))
	sig := hex.EncodeToString(mac.Sum(nil))

	u := &url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(absPath),
	}
	q := u.Query()
	q.Set("expires", strconv.FormatInt(expires, 10))
	q.Set("sig", sig)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
