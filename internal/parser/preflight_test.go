//go:build !integration

package parser

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreflight_OK(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "ok.zip")

	f, err := os.Create(path)
	require.NoError(t, err)

	zw := zip.NewWriter(f)
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())

	require.NoError(t, New().Preflight(path))
}

func TestPreflight_NotFound(t *testing.T) {
	t.Parallel()

	err := New().Preflight(filepath.Join(t.TempDir(), "missing.zip"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInputNotFound)
}

func TestPreflight_NotZip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "plain.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o600))

	err := New().Preflight(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInputNotZip)
}

func TestPreflight_Directory(t *testing.T) {
	t.Parallel()

	err := New().Preflight(t.TempDir())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInputNotZip)
}
