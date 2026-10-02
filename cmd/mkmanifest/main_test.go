package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ygelfand/LANovo/internal/update"
)

func TestTheManifestNamesTheArmBuildAndADeviceAcceptsIt(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "lanovod")
	if err := os.WriteFile(bin, []byte("thirty two"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "manifest.json")

	if err := run(update.Manifest{Version: "0.0.7"}, "https://example/download/0.0.7", map[string]string{"arm": bin}, out); err != nil {
		t.Fatal(err)
	}

	encoded, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var m update.Manifest
	if err := json.Unmarshal(encoded, &m); err != nil {
		t.Fatal(err)
	}
	if err := m.Valid(); err != nil {
		t.Fatal(err)
	}
	b, err := m.For("arm")
	if err != nil {
		t.Fatal(err)
	}
	if b.URL != "https://example/download/0.0.7/lanovod" || b.Size != 10 {
		t.Errorf("offered %+v", b)
	}
}

func TestAManifestWithoutTheArmBuildIsRefused(t *testing.T) {
	if err := run(update.Manifest{Version: "0.0.7"}, "https://example", map[string]string{}, ""); err == nil {
		t.Error("wrote a manifest no device can install from")
	}
}
