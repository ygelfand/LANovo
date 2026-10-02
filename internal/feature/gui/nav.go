package gui

import (
	"slices"
	"sync"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/feature/shell"
)

type Screen struct {
	Title string
	Build func(w *gogui.Window) gogui.View
	Clear bool
	View  shell.View
}

type Overlay struct {
	Priority int
	Build    func(w *gogui.Window) gogui.View
}

type Nav struct {
	For func(shell.View) *Screen

	mu       sync.Mutex
	screens  map[shell.View]*Screen
	overlays []*Overlay
}

func (n *Nav) resolve(stack []shell.View) []*Screen {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.screens == nil {
		n.screens = map[shell.View]*Screen{}
	}
	for v := range n.screens {
		if !slices.Contains(stack, v) {
			delete(n.screens, v)
		}
	}
	out := make([]*Screen, 0, len(stack))
	for _, v := range stack {
		s, ok := n.screens[v]
		if !ok && n.For != nil {
			s = n.For(v)
			n.screens[v] = s
		}
		if s != nil {
			out = append(out, s)
		}
	}
	return out
}

func (n *Nav) Showing(stack []shell.View) (covering *Screen, above []*Screen) {
	shown := n.resolve(shell.Showing(stack))
	if len(shown) > 0 && shown[0].View.Covers() {
		return shown[0], shown[1:]
	}
	return nil, shown
}

func (n *Nav) Show(o *Overlay) {
	n.mu.Lock()
	if !slices.Contains(n.overlays, o) {
		n.overlays = append(n.overlays, o)
		slices.SortStableFunc(n.overlays, func(a, b *Overlay) int { return a.Priority - b.Priority })
	}
	n.mu.Unlock()
}

func (n *Nav) Hide(o *Overlay) {
	n.mu.Lock()
	n.overlays = slices.DeleteFunc(n.overlays, func(x *Overlay) bool { return x == o })
	n.mu.Unlock()
}

func (n *Nav) Overlays() []*Overlay {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.Clone(n.overlays)
}
