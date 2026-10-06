package config

import (
	"github.com/ygelfand/libcountertop/pkg/audio/ducking"
	"os"
	"testing"
)

func TestOldSettingsKeepTheirEffectiveDuckingUntilConfigured(t *testing.T) {
	path := t.TempDir() + "/state.json"
	os.WriteFile(path, []byte(`{"volume":{"media":40}}`), 0600)
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := ducking.Gain(s.Get().Media.DuckDB); got != float32(0.2) {
		t.Fatal(got)
	}
	if err := s.Set().Media().DuckDB(-20); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := ducking.Gain(loaded.Get().Media.DuckDB); got != float32(0.1) {
		t.Fatal(got)
	}
}
