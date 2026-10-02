package pair

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// A Store on disk, so a phone that paired yesterday still connects today.
//
// Its own file rather than a corner of the settings, following the ESPHome key: these are secrets,
// and the settings document is read back out over the API and into diagnostics. One line per phone,
// address and key, because a file somebody may have to look at during a bad evening is worth being
// able to read.

// File is link keys kept on disk.
type File struct {
	path string

	mu   sync.RWMutex
	keys map[Addr]Key

	// readable is whether what was there was understood. It gates writing: everything is written
	// as one whole file, so a store that fell back to empty would replace a file it could not read
	// with one holding nothing, turning a bad read into every phone in the house re-pairing.
	//
	// A missing file is readable. There is nothing to lose and the first pairing creates it.
	readable bool
}

// Open reads the bonds at a path. A file that is not there is not an error — it is a device nobody
// has paired with yet.
//
// A file that does not parse comes back with the error and a store that refuses to write, so the
// keys that are in it survive to be looked at rather than being overwritten with nothing.
func Open(path string) (*File, error) {
	f := &File{path: path, keys: map[Addr]Key{}, readable: true}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return f, nil
	}
	if err != nil {
		f.readable = false
		return f, fmt.Errorf("pair: %s: %w", path, err)
	}

	keys, err := parseBonds(data)
	if err != nil {
		f.readable = false
		return f, fmt.Errorf("pair: %s: %w", path, err)
	}

	f.keys = keys
	return f, nil
}

// parseBonds reads the file's lines. Blank lines and comments are skipped so somebody can annotate
// one by hand without it being refused the next time it is read.
func parseBonds(data []byte) (map[Addr]Key, error) {
	out := map[Addr]Key{}

	s := bufio.NewScanner(bytes.NewReader(data))
	for line := 1; s.Scan(); line++ {
		text := strings.TrimSpace(s.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}

		addr, key, ok := strings.Cut(text, " ")
		if !ok {
			return nil, fmt.Errorf("line %d is not an address and a key", line)
		}

		a, err := decodeAddr(addr)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}

		raw, err := hex.DecodeString(strings.TrimSpace(key))
		if err != nil {
			return nil, fmt.Errorf("line %d: the key: %w", line, err)
		}
		if len(raw) != len(Key{}) {
			return nil, fmt.Errorf("line %d: a key of %d bytes, want %d",
				line, len(raw), len(Key{}))
		}

		out[a] = Key(raw)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// decodeAddr reads an address written the way people write them, most significant byte first.
func decodeAddr(s string) (Addr, error) {
	var a Addr

	parts := strings.Split(s, ":")
	if len(parts) != len(a) {
		return a, fmt.Errorf("%q is not an address", s)
	}

	for i, p := range parts {
		raw, err := hex.DecodeString(p)
		if err != nil || len(raw) != 1 {
			return a, fmt.Errorf("%q is not an address", s)
		}
		// Written most significant first, held the way it goes on the wire.
		a[len(a)-1-i] = raw[0]
	}
	return a, nil
}

func (f *File) Key(a Addr) (Key, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	k, ok := f.keys[a]
	return k, ok
}

func (f *File) Save(a Addr, k Key) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Present and unchanged, not merely equal to what a missing entry reads as — those are the
	// same comparison for an all-zero key, and taking them for the same thing means such a key is
	// never written down.
	if cur, ok := f.keys[a]; ok && cur == k {
		return nil
	}

	f.keys[a] = k
	return f.write()
}

func (f *File) Forget(a Addr) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.keys[a]; !ok {
		return nil
	}

	delete(f.keys, a)
	return f.write()
}

// Bonded is every address with a key, in a stable order, for a settings screen that lists them.
func (f *File) Bonded() []Addr {
	f.mu.RLock()
	defer f.mu.RUnlock()

	out := slices.Collect(maps.Keys(f.keys))
	slices.SortFunc(out, func(a, b Addr) int { return slices.Compare(a[:], b[:]) })
	return out
}

// write replaces the file through a temporary one. Called with the lock held.
//
// Mode 0600: a link key is what lets something claim to be a phone this device trusts, and there is
// no reason for anything else on the device to read it.
func (f *File) write() error {
	if !f.readable {
		return fmt.Errorf("pair: not writing over %s, which could not be read", f.path)
	}

	var b bytes.Buffer
	for _, a := range slices.SortedFunc(maps.Keys(f.keys), func(x, y Addr) int {
		return slices.Compare(x[:], y[:])
	}) {
		key := f.keys[a]
		fmt.Fprintf(&b, "%s %s\n", a, hex.EncodeToString(key[:]))
	}

	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("pair: %w", err)
	}

	// A name of its own rather than one fixed name, so two writes cannot share the file being
	// renamed into place.
	tmp, err := os.CreateTemp(dir, filepath.Base(f.path)+".*")
	if err != nil {
		return fmt.Errorf("pair: %s: %w", f.path, err)
	}
	name := tmp.Name()

	if err := writeThrough(tmp, b.Bytes()); err != nil {
		os.Remove(name)
		return fmt.Errorf("pair: %s: %w", name, err)
	}

	if err := os.Rename(name, f.path); err != nil {
		os.Remove(name)
		return fmt.Errorf("pair: %s: %w", f.path, err)
	}

	// The rename is what has to reach the disk, not only the file it replaced. This has to survive
	// a reboot, which is the thing that happens between pairing and reconnecting.
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
	return nil
}

func writeThrough(f *os.File, data []byte) error {
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
