package config

import (
	"path/filepath"
	"testing"
)

func fresh(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "state.json")
	Use(path)
	return path
}
