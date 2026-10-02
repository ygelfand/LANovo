// Package drawer is the rail of icons a swipe from the edge brings in.
//
// It is a way in rather than a place: nothing is set from here. Each icon opens a screen with room
// to do its job properly, and the rail is narrow enough that whatever the dashboard is showing
// carries on beside it.
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

// Where each thing sits on the rail. Declared here rather than left to whichever feature happens
// to be built first, which is decided by the import graph: adding an unrelated component once
// moved the player from the bottom of the rail to the top, and a rail is learned by position.
//
// Spaced out so something can be put between two of them without renumbering.
//
// These are the order the rail has had all along, written down rather than changed: declaring it
// is the fix, and picking a different one is a separate decision. Anything new goes after Player
// unless there is a reason to put it somewhere.
const (
	OrderSettings = 30
	OrderPlayer   = 40
	OrderIdle     = 50
)

// Entry is one icon on the rail and what it opens.
type Entry struct {
	// Name goes under the mark. A glyph on its own is a guess at what it opens; two of these are
	// speaker marks and they lead to different places.
	//
	// Asked for each time it is drawn, as Icon is, because the rail is built once at start-up and
	// the language can change after that. A name resolved at registration stays in the language it
	// was registered in until the next restart.
	Name func() string

	// Order is where it sits. Lower comes first; equal orders fall back to the name, so two
	// features that pick the same number still land the same way every boot.
	Order int

	Glyph func() string

	Open func()
}

// Label is the name to draw, or empty for an entry that has none.
func (e Entry) Label() string {
	if e.Name == nil {
		return ""
	}
	return e.Name()
}

// Rail is the strip itself.
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

// Add puts an icon on the rail. A feature offers its own way in rather than the rail knowing what
// the device can do, and says where it goes rather than taking whatever place it was built in.
func (r *Rail) Add(e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
}

// Entries is what is on the rail, in the order it is drawn and touched.
//
// Sorted here rather than on the way in, because a feature can be built at any point and sorting
// once at the end is the only way the answer does not depend on when each one arrived.
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

// on opens the rail when a finger comes in from the chosen edge.
func (r *Rail) on(g touch.Gesture) {
	if shell.Get().Open() || shell.Get().Closing() {
		return
	}
	if g.Kind == touch.Swipe && g.From == asTouch(config.Get().Screen.Drawer) {
		shell.Get().Push(r)
	}
}

// Covers is false: the rail hangs off one edge and the dashboard shows beside it.
func (r *Rail) Covers() bool { return false }

// asTouch is the same edge as the touchscreen names it.
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
