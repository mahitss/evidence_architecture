package storage

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorage_RoundTrip(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ls, err := NewLocalStorage(root)
	require.NoError(t, err)

	ctx := context.Background()
	key := "cases/case-1/evidence/ev-1/sample_doc.pdf"
	content := []byte("%PDF-1.4\nhello local storage\n")

	signedURL, err := ls.Put(ctx, key, bytes.NewReader(content), "application/pdf")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(signedURL, "file://"))
	assert.Contains(t, signedURL, "expires=")
	assert.Contains(t, signedURL, "sig=")

	rc, err := ls.Get(ctx, key)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, content, got)

	u, err := ls.URL(ctx, key, 2*time.Minute)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(u, "file://"))

	require.NoError(t, ls.Delete(ctx, key))

	_, err = ls.Get(ctx, key)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestLocalStorage_RejectsPathTraversal(t *testing.T) {
	t.Parallel()

	ls, err := NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	ctx := context.Background()
	badKeys := []string{
		"../secret.pdf",
		"cases/../../etc/passwd",
		"/etc/passwd",
		"cases/foo/../bar.pdf",
		`cases\foo\bar.pdf`,
		"cases/bad space.pdf",
		"",
	}

	for _, k := range badKeys {
		_, putErr := ls.Put(ctx, k, strings.NewReader("data"), "application/pdf")
		require.Error(t, putErr, "key %q must be rejected on Put", k)
		assert.ErrorIs(t, putErr, ErrInvalidKey)

		_, getErr := ls.Get(ctx, k)
		require.Error(t, getErr, "key %q must be rejected on Get", k)
		assert.ErrorIs(t, getErr, ErrInvalidKey)

		delErr := ls.Delete(ctx, k)
		require.Error(t, delErr, "key %q must be rejected on Delete", k)
		assert.ErrorIs(t, delErr, ErrInvalidKey)
	}
}

func TestLocalStorage_RejectsDisallowedContentType(t *testing.T) {
	t.Parallel()

	ls, err := NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	ctx := context.Background()
	disallowed := []string{
		"text/html",
		"application/javascript",
		"application/x-executable",
		"application/octet-stream",
		"",
	}

	for _, ct := range disallowed {
		_, err := ls.Put(ctx, "cases/c1/file.bin", strings.NewReader("payload"), ct)
		require.Error(t, err, "content-type %q must be rejected", ct)
		assert.ErrorIs(t, err, ErrUnsupportedContentType)
	}

	// Verify every allowlisted MIME type succeeds
	allowed := []string{
		"application/pdf",
		"image/jpeg",
		"image/png",
		"image/webp",
		"text/plain",
		"application/json",
	}
	for i, ct := range allowed {
		key := "cases/c1/ok_" + strings.ReplaceAll(ct, "/", "_") + "_" + string(rune('a'+i))
		_, err := ls.Put(ctx, key, strings.NewReader("ok"), ct)
		require.NoError(t, err, "content-type %q must be allowed", ct)
	}
}

func TestLocalStorage_EnforcesSizeLimit(t *testing.T) {
	t.Parallel()

	ls, err := NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	ls.SetMaxBytes(1024) // 1 KB cap for fast test verification

	ctx := context.Background()

	// Exactly at limit succeeds
	_, err = ls.Put(ctx, "cases/c1/exact.txt", bytes.NewReader(bytes.Repeat([]byte("a"), 1024)), "text/plain")
	require.NoError(t, err)

	// 1 byte over limit fails cleanly without leaving committed object
	_, err = ls.Put(ctx, "cases/c1/oversize.txt", bytes.NewReader(bytes.Repeat([]byte("b"), 1025)), "text/plain")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFileTooLarge)

	_, getErr := ls.Get(ctx, "cases/c1/oversize.txt")
	assert.ErrorIs(t, getErr, ErrNotFound)
}
