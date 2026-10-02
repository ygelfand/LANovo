package config

import (
	"path/filepath"
	"testing"
)

// fresh points the config at an empty file for one test.
func fresh(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "state.json")
	Use(path)
	return path
}
