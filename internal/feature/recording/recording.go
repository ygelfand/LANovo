package recording

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/lib/wave"
)

const Slots = component.Assistants

var dir = layout.RecordingDir

func init() {
	component.Register(component.Device, Get, component.Order(20))
}

// The transport caps a message at 65515 bytes; base64 adds a third.
const Page = 32 * 1024

const Longest = 30 * mic.Voice * 2

const KeepMost = 10

const (
	wavExt  = ".wav"
	metaExt = ".json"
)

type meta struct {
	Slot  int `json:"slot"`
	Bytes int `json:"bytes"`

	At int64 `json:"at"`
}

type Store struct {
	mu   sync.Mutex
	open string
	slot int
	buf  []byte

	disk sync.Mutex

	keep []*esphome.Number
}

var (
	once   sync.Once
	shared *Store
)

func Get() *Store {
	once.Do(func() {
		shared = &Store{}

		for n := range Slots {
			slot := n
			number := &esphome.Number{
				Base: esphome.Base{
					ObjectID: fmt.Sprintf("keep_recordings_%d", slot+1),
					Name:     "Recordings kept",
					Icon:     "mdi:record-rec",
					DeviceID: component.AssistantDevice(slot),
					Category: esphome.CategoryConfig,
				},
				Min: 0, Max: KeepMost, Step: 1,
				Mode: esphome.NumberBox,
			}

			number.OnCommand = func(v float32) {
				number.Set(v)
				if err := config.Set().Wake(slot).Recordings(int(v)); err != nil {
					slog.Error(
						"saving how many recordings to keep failed",
						"slot",
						slot+1,
						"err",
						err,
					)
				}
				shared.Prune()
			}
			shared.keep = append(shared.keep, number)
		}
	})
	return shared
}

func (s *Store) Name() string { return "recording" }

func (s *Store) Entities() []esphome.Entity {
	out := make([]esphome.Entity, 0, len(s.keep))
	for _, number := range s.keep {
		out = append(out, number)
	}
	return out
}

func (s *Store) Restore(c config.Config) {
	for slot, number := range s.keep {
		count := 0
		if slot < len(c.Wake.Words) {
			count = c.Wake.Words[slot].Recordings
		}
		number.Set(float32(count))
	}
	s.Prune()
}

func (s *Store) Actions() []*esphome.Action {
	return []*esphome.Action{
		{
			Name:    "recordings",
			Answers: true,
			Run: func(esphome.Call) (any, error) {
				return s.available()
			},
		},
		{
			Name: "turn_audio",
			Args: []esphome.Arg{
				{Name: "id", Type: esphome.ArgString},
				{Name: "page", Type: esphome.ArgInt},
			},
			Answers: true,
			Run: func(c esphome.Call) (any, error) {
				return s.page(c.String("id"), c.Int("page"))
			},
		},
	}
}

type Available struct {
	Version int      `json:"version"`
	IDs     []string `json:"ids"`
}

func (s *Store) available() (*Available, error) {
	out := &Available{Version: 1, IDs: []string{}}

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if name := e.Name(); strings.HasSuffix(name, metaExt) {
			out.IDs = append(out.IDs, strings.TrimSuffix(name, metaExt))
		}
	}
	return out, nil
}

func (s *Store) Opens(id string, slot int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if keeps(slot) <= 0 {
		s.open, s.buf = "", nil
		return
	}
	s.open, s.slot, s.buf = id, slot, nil
}

func (s *Store) Frame(pcm []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.open == "" || len(s.buf) >= Longest {
		return
	}
	s.buf = append(s.buf, pcm...)
}

func (s *Store) Closes() {
	s.mu.Lock()
	id, slot, buf := s.open, s.slot, s.buf
	s.open, s.buf = "", nil
	s.mu.Unlock()

	if id == "" || len(buf) == 0 {
		return
	}

	s.disk.Lock()
	defer s.disk.Unlock()

	if err := write(id, slot, buf); err != nil {
		slog.Error("saving a recording failed", "id", id, "err", err)
		return
	}
	s.prune()
}

func (s *Store) Seconds(id string) float64 {
	m, err := readMeta(id)
	if err != nil {
		return 0
	}
	return float64(m.Bytes) / float64(mic.Voice*2)
}

// The frontend rejects an answer without Version.
type Answer struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
	Page    int    `json:"page"`
	Pages   int    `json:"pages"`
	MIME    string `json:"mime"`
	Data    string `json:"data"`
}

func (s *Store) page(id string, page int) (*Answer, error) {
	whole, err := os.ReadFile(wavPath(id))
	if err != nil {
		return nil, fmt.Errorf("no recording for turn %s", id)
	}

	pages := (len(whole) + Page - 1) / Page
	if page < 0 || page >= pages {
		return nil, fmt.Errorf("page %d of %d", page, pages)
	}

	end := min((page+1)*Page, len(whole))
	return &Answer{
		Version: 1,
		ID:      id,
		Page:    page,
		Pages:   pages,
		MIME:    "audio/wav",
		Data:    base64.StdEncoding.EncodeToString(whole[page*Page : end]),
	}, nil
}

func (s *Store) Prune() {
	s.disk.Lock()
	defer s.disk.Unlock()
	s.prune()
}

func (s *Store) prune() {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		slog.Error("reading the recordings failed", "dir", dir, "err", err)
		return
	}

	type rec struct {
		id string
		at int64
	}
	bySlot := map[int][]rec{}
	complete := map[string]bool{}
	wavs := []string{}

	for _, e := range entries {
		switch name := e.Name(); {
		case strings.HasSuffix(name, metaExt):
			id := strings.TrimSuffix(name, metaExt)
			m, err := readMeta(id)
			if err != nil {
				continue
			}
			complete[id] = true
			bySlot[m.Slot] = append(bySlot[m.Slot], rec{id, m.At})
		case strings.HasSuffix(name, wavExt):
			wavs = append(wavs, strings.TrimSuffix(name, wavExt))
		}
	}

	for slot, recs := range bySlot {
		sort.Slice(recs, func(i, j int) bool {
			if recs[i].at != recs[j].at {
				return recs[i].at > recs[j].at
			}
			return recs[i].id > recs[j].id
		})
		for i, r := range recs {
			if i >= keeps(slot) {
				remove(r.id)
			}
		}
	}

	for _, id := range wavs {
		if !complete[id] {
			remove(id)
		}
	}
}

func keeps(slot int) int {
	words := config.Get().Wake.Words
	if slot < 0 || slot >= len(words) {
		return 0
	}
	return max(words[slot].Recordings, 0)
}

func write(id string, slot int, pcm []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(wavPath(id), wave.Mono16(pcm, mic.Voice), 0o644); err != nil {
		return err
	}

	blob, err := json.Marshal(meta{Slot: slot, Bytes: len(pcm), At: time.Now().UnixNano()})
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath(id), blob, 0o644)
}

func remove(id string) {
	for _, path := range []string{wavPath(id), metaPath(id)} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			slog.Error("removing a recording failed", "path", path, "err", err)
		}
	}
}

func readMeta(id string) (meta, error) {
	blob, err := os.ReadFile(metaPath(id))
	if err != nil {
		return meta{}, err
	}
	var m meta
	return m, json.Unmarshal(blob, &m)
}

func wavPath(id string) string  { return filepath.Join(dir, id+wavExt) }
func metaPath(id string) string { return filepath.Join(dir, id+metaExt) }
