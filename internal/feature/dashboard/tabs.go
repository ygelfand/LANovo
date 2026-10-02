package dashboard

import (
	"sync"

	"github.com/ygelfand/LANovo/internal/feature/shell"
)

type Tab struct {
	Kind string
	Key  string
	Name string
}

type tabs struct {
	mu      sync.Mutex
	sources []func() []Tab
	showing string
	redraw  func()
}

var strip = &tabs{redraw: func() { shell.Get().Redraw() }}

func AddTabs(source func() []Tab) {
	strip.mu.Lock()
	strip.sources = append(strip.sources, source)
	strip.mu.Unlock()
}

func Tabs() []Tab { return strip.list() }

func Showing() (Tab, bool) { return strip.open() }

func Show(key string) { strip.show(key) }

func Clock() { strip.clock() }

func (t *tabs) list() []Tab {
	t.mu.Lock()
	sources := t.sources
	t.mu.Unlock()
	var out []Tab
	for _, source := range sources {
		out = append(out, source()...)
	}
	return out
}

func (t *tabs) open() (Tab, bool) {
	t.mu.Lock()
	key := t.showing
	t.mu.Unlock()
	if key == "" {
		return Tab{}, false
	}
	for _, tab := range t.list() {
		if tab.Key == key {
			return tab, true
		}
	}
	return Tab{}, false
}

func (t *tabs) show(key string) {
	t.mu.Lock()
	if t.showing == key {
		key = ""
	}
	t.showing = key
	t.mu.Unlock()
	t.redraw()
}

func (t *tabs) clock() {
	t.mu.Lock()
	changed := t.showing != ""
	t.showing = ""
	t.mu.Unlock()
	if changed {
		t.redraw()
	}
}
