package api

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	esphome "github.com/ygelfand/go-esphome-device"
)

func newKey(t *testing.T) esphome.PSK {
	t.Helper()

	k, err := esphome.GeneratePSK()
	if err != nil {
		t.Fatalf("GeneratePSK: %v", err)
	}
	return k
}

func TestNoKeyIsUnprovisioned(t *testing.T) {
	psk, err := loadPSK(filepath.Join(t.TempDir(), "psk"))
	if err != nil {
		t.Fatalf("loadPSK: %v", err)
	}
	if psk == nil {
		t.Fatal("loadPSK gave no key at all")
	}
	if !psk.IsZero() {
		t.Error("a device with no key invented one instead of coming up unprovisioned")
	}
}

func TestUnprovisionedWritesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "psk")

	if _, err := loadPSK(path); err != nil {
		t.Fatalf("loadPSK: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a key file appeared at %s: %v", path, err)
	}
}

func TestKeyRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "psk")

	want := newKey(t)
	if err := writePSK(path, want); err != nil {
		t.Fatalf("writePSK: %v", err)
	}

	got, err := loadPSK(path)
	if err != nil {
		t.Fatalf("loadPSK: %v", err)
	}
	if got.IsZero() || *got != want {
		t.Errorf("the key came back as %v, want %v", got, want)
	}
}

func TestKeyIgnoresTrailingWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "psk")

	want := newKey(t)
	if err := os.WriteFile(path, []byte(want.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := loadPSK(path)
	if err != nil {
		t.Fatalf("loadPSK: %v", err)
	}
	if *got != want {
		t.Errorf("the key came back as %v", got)
	}
}

func TestUnreadableKeyIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "psk")
	if err := os.WriteFile(path, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := loadPSK(path); err == nil {
		t.Error("a corrupt key file was accepted")
	}
}

func TestKeyIsWrittenPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "psk")

	if err := writePSK(path, newKey(t)); err != nil {
		t.Fatalf("writePSK: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		t.Errorf("the key is mode %o, want it readable only by its owner", mode)
	}
}

func TestKeyCreatesItsDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "made", "up", "psk")

	if err := writePSK(path, newKey(t)); err != nil {
		t.Fatalf("writePSK: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the key was not written: %v", err)
	}
}
