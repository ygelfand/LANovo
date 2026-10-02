package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/ygelfand/LANovo/internal/layout"
)

// Store holds the settings and writes them down.
type Store struct {
	path string

	// writes serializes a change and the file it produces. Two that overlapped would write the
	// same temporary file over each other and rename whatever the interleaving left, and a file
	// that does not parse is one this store will then refuse to write to at all.
	writes sync.Mutex

	mu  sync.RWMutex
	cfg Config

	// readable is whether what is on disk was understood. It gates writing, because everything here
	// is written as one whole document: a store that fell back to defaults would replace a file it
	// could not read with a file that has nothing in it, turning a bad read into permanent loss.
	//
	// A missing file is readable. There is nothing to lose and the first change creates it.
	readable bool
}

// The process-wide store. There is one device and one file, so callers use the package functions
// rather than being handed a Store.
var (
	once    sync.Once
	shared  *Store
	loadErr error
)

func store() *Store {
	once.Do(func() { shared, loadErr = Load(layout.StatePath) })
	return shared
}

// Get is everything the device is set to.
func Get() Config { return store().Get() }

// Set names what is being changed and persists it.
func Set() Writer { return store().Set() }

// LoadError is what reading the saved settings said, for a caller that wants to report it. The
// defaults are in use either way.
func LoadError() error { store(); return loadErr }

// Started records what this process was told, so nothing has to be handed a struct to find out what
// the device is called or where it listens.
func Started(d Device) { store().started(d) }

// Use points the process-wide store at another path. For tests.
func Use(path string) {
	once.Do(func() {})
	shared, loadErr = Load(path)
}

// Load reads a settings file, falling back to the defaults for anything it does not mention.
func Load(path string) (*Store, error) {
	st := &Store{path: path, cfg: Defaults(), readable: true}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		st.readable = false
		return st, fmt.Errorf("config: %s: %w", path, err)
	}

	if err := json.Unmarshal(data, &st.cfg); err != nil {
		st.readable = false
		st.cfg = Defaults()
		return st, fmt.Errorf("config: %s: %w", path, err)
	}
	return st, nil
}

func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *Store) Set() Writer { return Writer{st: s} }

func (s *Store) started(d Device) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.Device = d
}

// Update changes one setting and writes the file.
//
// The change and the write are one operation, so the file that ends up on disk is the newest state
// rather than whichever writer happened to finish last.
func (s *Store) Update(change func(*Config)) error {
	s.writes.Lock()
	defer s.writes.Unlock()

	s.mu.Lock()
	change(&s.cfg)
	cfg, readable := s.cfg, s.readable
	s.mu.Unlock()

	if !readable {
		return fmt.Errorf("config: not writing over %s, which could not be read", s.path)
	}
	return s.write(cfg)
}

// write replaces the file through a temporary one, flushed at every step, because the thing this
// has to survive is a reboot rather than a crash.
//
// A rename is atomic, which is enough for being killed: either the old file or the new one is
// there.
func (s *Store) write(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// A name of its own rather than one fixed name. Anything else writing beside this would
	// otherwise share the file being renamed into place, and what arrives is whichever write the
	// interleaving left.
	f, err := os.CreateTemp(filepath.Dir(s.path), filepath.Base(s.path)+".*")
	if err != nil {
		return fmt.Errorf("config: %s: %w", s.path, err)
	}
	tmp := f.Name()

	if err := f.Chmod(0o644); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("config: %s: %w", tmp, err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("config: %s: %w", tmp, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("config: %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("config: %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("config: %s: %w", s.path, err)
	}

	// The rename is what has to reach the disk, not just the file it replaced.
	if dir, err := os.Open(filepath.Dir(s.path)); err == nil {
		dir.Sync()
		dir.Close()
	}
	return nil
}
