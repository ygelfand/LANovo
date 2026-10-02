package pair

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bonds(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "bonds")
}

// A device nobody has paired with yet has no file, which is not a fault.
func TestNoFileYet(t *testing.T) {
	f, err := Open(bonds(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, ok := f.Key(device); ok {
		t.Error("a key came back from a file that does not exist")
	}
}

// The whole point: a key saved is a key that is still there after a reboot.
func TestAKeySurvivesBeingReopened(t *testing.T) {
	path := bonds(t)
	key := Key{0: 0x01, 7: 0x80, 15: 0xff}

	f, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := f.Save(device, key); err != nil {
		t.Fatalf("Save: %v", err)
	}

	again, err := Open(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}

	got, ok := again.Key(device)
	if !ok {
		t.Fatal("the key did not come back")
	}
	if got != key {
		t.Errorf("came back %x, want %x", got, key)
	}
}

// An address is held backwards from how it is written. Writing it one way and reading it the other
// makes a phone that paired yesterday a stranger today, which is the bug this is here to catch.
func TestTheAddressRoundTripsThroughTheFile(t *testing.T) {
	path := bonds(t)

	f, _ := Open(path)
	if err := f.Save(device, Key{0: 0x2a}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	if !strings.HasPrefix(string(data), "66:55:44:33:22:11 ") {
		t.Errorf("the file starts %q, want the address as it is written", string(data))
	}

	again, _ := Open(path)
	if _, ok := again.Key(device); !ok {
		t.Error("the address did not survive the round trip")
	}
}

func TestForgettingRemovesItFromTheFile(t *testing.T) {
	path := bonds(t)

	f, _ := Open(path)
	f.Save(device, Key{0: 1})
	f.Save(Addr{0x09}, Key{0: 2})

	if err := f.Forget(device); err != nil {
		t.Fatalf("Forget: %v", err)
	}

	again, err := Open(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	if _, ok := again.Key(device); ok {
		t.Error("the forgotten key came back")
	}
	if _, ok := again.Key(Addr{0x09}); !ok {
		t.Error("forgetting one dropped the other")
	}
}

// A file that could not be read is not overwritten. Replacing it with an empty one turns a bad
// read into every phone in the house pairing again.
func TestAFileThatDoesNotParseIsNotOverwritten(t *testing.T) {
	path := bonds(t)
	if err := os.WriteFile(path, []byte("this is not a bond\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	f, err := Open(path)
	if err == nil {
		t.Error("a file that does not parse was read without complaint")
	}

	if err := f.Save(device, Key{0: 1}); err == nil {
		t.Error("a store that could not read its file wrote to it anyway")
	}

	data, _ := os.ReadFile(path)
	if string(data) != "this is not a bond\n" {
		t.Errorf("the file was replaced with %q", string(data))
	}
}

func TestWhatTheFileWillNotAccept(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"no key", "66:55:44:33:22:11\n"},
		{"a short key", "66:55:44:33:22:11 0102\n"},
		{"a key that is not hex", "66:55:44:33:22:11 " + strings.Repeat("zz", 16) + "\n"},
		{"a short address", "55:44:33:22:11 " + strings.Repeat("00", 16) + "\n"},
		{"an address that is not hex", "gg:55:44:33:22:11 " + strings.Repeat("00", 16) + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := bonds(t)
			os.WriteFile(path, []byte(tc.body), 0o600)

			if _, err := Open(path); err == nil {
				t.Errorf("%q was accepted", tc.body)
			}
		})
	}
}

// Blank lines and comments so somebody can annotate one by hand during a bad evening without the
// device refusing to read it afterwards.
func TestCommentsAndBlankLinesAreSkipped(t *testing.T) {
	path := bonds(t)
	body := "# the phone in the kitchen\n\n66:55:44:33:22:11 " + strings.Repeat("ab", 16) + "\n\n"
	os.WriteFile(path, []byte(body), 0o600)

	f, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, ok := f.Key(device); !ok {
		t.Error("the key was not read")
	}
}

// A link key is what lets something claim to be a phone this device trusts.
func TestTheFileIsNotReadableByAnythingElse(t *testing.T) {
	path := bonds(t)

	f, _ := Open(path)
	f.Save(device, Key{0: 1})

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("the bonds are mode %o", mode)
	}
}

func TestSavingTheSameKeyTwiceIsNotAWrite(t *testing.T) {
	path := bonds(t)
	key := Key{0: 0x11}

	f, _ := Open(path)
	f.Save(device, key)

	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.Save(device, key); err != nil {
		t.Fatalf("Save: %v", err)
	}

	after, _ := os.Stat(path)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("saving an unchanged key rewrote the file")
	}
}

// A phone that re-pairs sends a new key, and the new one is the one that works.
func TestRepairingReplacesTheKey(t *testing.T) {
	path := bonds(t)

	f, _ := Open(path)
	f.Save(device, Key{0: 0x01})
	f.Save(device, Key{0: 0x02})

	again, _ := Open(path)
	got, _ := again.Key(device)
	if got[0] != 0x02 {
		t.Errorf("the key came back %x, want the newer one", got)
	}
}

func TestBondedComesBackInAStableOrder(t *testing.T) {
	f, _ := Open(bonds(t))
	f.Save(device, Key{})
	f.Save(Addr{0x01}, Key{})

	got := f.Bonded()
	if len(got) != 2 {
		t.Fatalf("%d bonds", len(got))
	}
	if got[0] != (Addr{0x01}) {
		t.Errorf("came back starting with %v", got[0])
	}
}

// The file backs the same interface the manager takes, which is the only thing that makes it
// useful.
func TestAFileIsAStore(t *testing.T) {
	f, _ := Open(bonds(t))
	m := New(f)

	quiet(t, m, notification(device, Key{0: 0x5a}, KeyUnauthenticatedP256))

	if _, ok := f.Key(device); !ok {
		t.Error("pairing did not reach the file")
	}
}
