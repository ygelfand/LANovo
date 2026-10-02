package video

import (
	"sync"

	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/lib/surface"
)

type Beneath struct {
	Stalled func()
	Resumed func()

	mu   sync.Mutex
	c    *surface.Client
	kept uint32
	spec spec
}

type spec struct {
	codec   uint32
	w, h    int
	session uint32
}

func (b *Beneath) Orientation() display.Orientation { return display.Get().Orientation() }
func (b *Beneath) Native() (int, int)               { return display.Get().Native() }

func (b *Beneath) Stalling() {
	if b.Stalled != nil {
		b.Stalled()
	}
}

func (b *Beneath) Resuming() {
	if b.Resumed != nil {
		b.Resumed()
	}
}

func (b *Beneath) Reuse(c *surface.Client, s spec) (uint32, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.c != c || b.kept == 0 || b.spec != s {
		return 0, false
	}
	return b.kept, true
}

func (b *Beneath) Adopt(c *surface.Client, id uint32, s spec) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.c != nil && b.kept != 0 && b.kept != id {
		b.c.Destroy(b.kept)
	}
	b.c, b.kept, b.spec = c, id, s
}

func (b *Beneath) Supersede(id uint32) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.c != nil && b.kept != 0 && b.kept != id {
		b.c.Destroy(b.kept)
		b.c, b.kept = nil, 0
	}
}

func (b *Beneath) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.c != nil && b.kept != 0 {
		b.c.Destroy(b.kept)
	}
	b.c, b.kept = nil, 0
}
