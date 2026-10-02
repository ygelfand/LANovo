package videoplayer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/ui"
)

const (
	spinDots  = 12
	spinEvery = 100 * time.Millisecond
	spinAfter = 300 * time.Millisecond
)

// Page is a video on the shell: the picture shows through it, and its controls, spinner, logo
// and errors are drawn like any other screen.
type Page struct {
	c Controls

	mu      sync.Mutex
	picture bool
	loading bool
	began   time.Time
	note    string
	logo    *ui.Image
	frameW  int
	frameH  int
	shown   bool
	paused  bool
	until   time.Time
	drawn   string
	hold    *shell.Hold
}

func (p *Page) Shows(s config.Stream) bool { return s == config.StreamMedia }

func (p *Page) reveal() {
	p.mu.Lock()
	p.shown, p.until = true, time.Now().Add(linger)
	p.mu.Unlock()
	p.redraw()
}

type Look struct {
	Controls Controls
	Picture  bool
	Spinning bool
	Shown    bool
	Began    time.Time
	Note     string
	Logo     *ui.Image
	FrameW   int
	FrameH   int
}

func (p *Page) Look() Look {
	p.mu.Lock()
	defer p.mu.Unlock()
	return Look{
		Controls: p.c,
		Picture:  p.picture,
		Spinning: p.loading && time.Since(p.began) > spinAfter,
		Shown:    p.shown,
		Began:    p.began,
		Note:     p.note,
		Logo:     p.logo,
		FrameW:   p.frameW,
		FrameH:   p.frameH,
	}
}

func (p *Page) Reveal() { p.reveal() }

func (p *Page) Conceal() {
	p.mu.Lock()
	p.shown = false
	p.mu.Unlock()
	p.redraw()
}

func (p *Page) Linger() {
	p.mu.Lock()
	p.until = time.Now().Add(linger)
	p.mu.Unlock()
}

func NewPage(c Controls) *Page { return &Page{c: c, loading: true, began: time.Now()} }

func (p *Page) Covers() bool { return true }

// Follow points the controls at what plays now, when one track hands over to the next.
func (p *Page) Follow(c Controls) {
	p.mu.Lock()
	p.c = c
	p.mu.Unlock()
}

func (p *Page) controls() Controls {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.c
}

func (p *Page) SetPicture(on bool) {
	p.mu.Lock()
	p.picture = on
	if on {
		p.loading = false
	}
	up := p.hold.Held()
	p.mu.Unlock()
	p.redraw()
	if on && !up {
		p.Show()
	}
}

func (p *Page) Wakes() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.picture
}

func (p *Page) SetLoading(on bool) {
	p.mu.Lock()
	changed := on != p.loading
	if on && changed {
		p.began = time.Now()
	}
	p.loading = on
	p.mu.Unlock()
	if changed {
		p.redraw()
	}
}

func (p *Page) SetNote(note string) {
	p.mu.Lock()
	p.note = note
	if note != "" {
		p.loading = false
	}
	p.mu.Unlock()
	p.redraw()
}

func (p *Page) SetFrame(w, h int) {
	p.mu.Lock()
	p.frameW, p.frameH = w, h
	p.mu.Unlock()
	p.redraw()
}

func (p *Page) SetLogo(img *ui.Image) {
	p.mu.Lock()
	p.logo = img
	p.mu.Unlock()
	p.redraw()
}

func (p *Page) redraw() {
	if shell.Get().Top() == p {
		shell.Get().Redraw()
	}
}

// Run puts the page up and keeps it current until ctx ends.
func (p *Page) Run(ctx context.Context) {
	p.Show()
	quiet := volume.Get().Changed.Listen(func(c volume.Change) {
		if c.Stream == config.StreamMedia && shell.Get().Top() == shell.View(p) {
			p.reveal()
		}
	})
	defer quiet()
	defer func() {
		p.mu.Lock()
		hold := p.hold
		p.hold = nil
		p.mu.Unlock()
		hold.Release()
	}()
	t := time.NewTicker(spinEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.tick()
		}
	}
}

// Show puts the page back up, for coming back to it from the mini player.
func (p *Page) Show() {
	p.mu.Lock()
	held := p.hold.Held()
	p.mu.Unlock()
	if held {
		return
	}
	hold := shell.Get().Hold(p)
	p.mu.Lock()
	p.hold = hold
	p.mu.Unlock()
}

var watched atomic.Int64

func Watched() time.Time {
	at := watched.Load()
	if at == 0 {
		return time.Time{}
	}
	return time.Unix(0, at)
}

func (p *Page) tick() {
	now := p.controls().Now()
	p.mu.Lock()
	if p.picture && now.Playing && !now.Paused {
		watched.Store(time.Now().UnixNano())
	}
	if p.shown && now.Playing && !now.Paused && time.Now().After(p.until) {
		p.shown = false
	}
	if !p.shown && now.Paused && !p.paused {
		p.shown, p.until = true, time.Now().Add(linger)
	}
	p.paused = now.Paused
	spinning := p.loading && time.Since(p.began) > spinAfter
	key := fmt.Sprintf("%v|%v|%d|%v|%v|%s|%s|%s|%p|%p|%d|%d", p.shown, p.picture, now.Elapsed/time.Second, now.Paused, now.Playing, now.Title, now.Artist, now.Album, now.Art, now.Mark, now.Can, volume.Get().Level(config.StreamMedia))
	changed := key != p.drawn
	p.drawn = key
	p.mu.Unlock()
	if spinning || changed {
		p.redraw()
	}
}
