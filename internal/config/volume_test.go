package config

import (
	"os"
	"testing"
)

func TestASavedFileWithoutMainKeepsItsMediaLevel(t *testing.T) {
	path := t.TempDir() + "/state.json"
	if err := os.WriteFile(path, []byte(`{"volume":{"media":25,"voice":35}}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	v := s.Get().Volume
	if v.Main != DefaultMainVolume || v.Media != 25 || v.Voice != 35 {
		t.Errorf("loaded %+v, want main at %d and the saved levels", v, DefaultMainVolume)
	}
}

func TestEveryStreamSavesToItsOwnLevel(t *testing.T) {
	s, err := Load(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatal(err)
	}
	for i, stream := range Streams() {
		if err := s.Set().Volume().Level(stream, 10+i); err != nil {
			t.Fatal(err)
		}
	}
	for i, stream := range Streams() {
		if got := s.Get().Volume.Level(stream); got != 10+i {
			t.Errorf("%s saved as %d, want %d", stream, got, 10+i)
		}
	}
}
