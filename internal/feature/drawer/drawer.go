package drawer

import (
	"cmp"
	"slices"
	"sync"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/touch"
)

func init() {
	component.Register(component.Device, Get, component.Order(35))
}

const (
	OrderSettings = 30
	OrderPlayer   = 40
	OrderIdle     = 50
)

type Entry struct {
	Name func() string

	Order int

	Glyph func() string

	Open func()
}

func (e Entry) Label() string {
	if e.Name == nil {
		return ""
	}
	return e.Name()
}

type Rail struct {
	mu      sync.Mutex
	entries []Entry
}

var (
	once   sync.Once
	shared *Rail
)

func Get() *Rail {
	once.Do(func() {
		shared = &Rail{}
		touch.Get().Gestures.Listen(shared.on)
	})
	return shared
}

func (r *Rail) Name() string { return "drawer" }

func (r *Rail) Add(e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
}

func (r *Rail) Entries() []Entry {
	r.mu.Lock()
	out := append([]Entry(nil), r.entries...)
	r.mu.Unlock()

	slices.SortStableFunc(out, func(a, b Entry) int {
		if v := cmp.Compare(a.Order, b.Order); v != 0 {
			return v
		}
		return cmp.Compare(a.Label(), b.Label())
	})
	return out
}

func (r *Rail) on(g touch.Gesture) {
	if shell.Get().Open() || shell.Get().Closing() {
		return
	}
	if g.Kind == touch.Swipe && g.From == asTouch(config.Get().Screen.Drawer) {
		shell.Get().Push(r)
	}
}

func (r *Rail) Covers() bool { return false }

func asTouch(e config.Edge) touch.Edge {
	switch e {
	case config.EdgeLeft:
		return touch.Left
	case config.EdgeTop:
		return touch.Top
	case config.EdgeBottom:
		return touch.Bottom
	}
	return touch.Right
}
