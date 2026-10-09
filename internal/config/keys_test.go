package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

func TestSavedKeysKeepTheirLayout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.Set().Screen().Backlight(schema.DefaultBacklight); err != nil {
		t.Fatalf("Backlight: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	want := []string{
		"api", "bluetooth", "call", "camera", "cast", "clock", "diag", "feedback", "home", "idle",
		"media", "microphone", "network", "poster", "rtsp", "screen", "sendspin", "time", "update",
		"visual", "volume", "wake", "weather", "access", "presence", "alerts",
	}
	slices.Sort(want)
	got := make([]string, 0, len(top))
	for key := range top {
		got = append(got, key)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("saved keys = %v, want %v", got, want)
	}
}
