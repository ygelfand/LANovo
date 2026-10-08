package config

import (
	"os"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/audio/ducking"
)

func TestCommonDuckingDefaultAndSavedSetting(t *testing.T) {
	path := t.TempDir() + "/state.json"
	if err := os.WriteFile(path, []byte(`{"volume":{"media":40}}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Get().Media.DuckDB; got != ducking.DefaultDB {
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
