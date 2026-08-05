package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoad(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "config.yaml")
	want := Default()
	want.Database.Path = "test.db"
	want.Strava.ClientID = "123"
	want.Strava.ExpiresAt = 1_700_000_000

	require.NoError(t, Save(filename, want))
	got, err := Load(filename)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	info, err := os.Stat(filename)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestSaveReplacesExistingConfigAtomically(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, Save(filename, Default()))
	updated := Default()
	updated.Server.Address = "127.0.0.1:9090"
	require.NoError(t, Save(filename, updated))

	got, err := Load(filename)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:9090", got.Server.Address)
}
