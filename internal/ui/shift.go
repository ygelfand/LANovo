package ui

import (
	"image"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Shift is a surface moved sideways: what is drawn at the origin lands at dx, dy instead, and
// anything that falls outside is discarded by the surface underneath.
//
// Size stays the surface's own, so whatever draws on it lays out for the whole picture and is then
// moved. That is what a page sliding in is: the same page, drawn off to one side.
func Shift(s Surface, dx, dy int) Surface { return shifted{to: s, dx: dx, dy: dy} }

// Every surface here reads, fills and clips, so the shift can carry all three without asking. A
// surface that did not would come back through these methods as one that does.
type shifted struct {
	to     Surface
	dx, dy int
}

func (s shifted) Size() (w, h int) { return s.to.Size() }

func (s shifted) Set(x, y int, c theme.Color) { s.to.Set(x+s.dx, y+s.dy, c) }

func (s shifted) DrawRGBA(x, y int, img *image.RGBA, scale int, clip Rect) {
	if clip.W > 0 && clip.H > 0 {
		clip.X, clip.Y = clip.X+s.dx, clip.Y+s.dy
	}
	DrawRGBA(s.to, x+s.dx, y+s.dy, img, scale, clip)
}

func (s shifted) At(x, y int) theme.Color {
	r, ok := s.to.(Reader)
	if !ok {
		return theme.Color{}
	}
	return r.At(x+s.dx, y+s.dy)
}

func (s shifted) FillRect(r Rect, c theme.Color) {
	FillRect(s.to, Rect{X: r.X + s.dx, Y: r.Y + s.dy, W: r.W, H: r.H}, c)
}

// Clipped is what the surface underneath is accepting, moved back into these coordinates, so text
// and rows outside it are skipped rather than drawn and thrown away.
//
// A surface accepting everything is still bounded by its own edges, and saying so is the whole
// point here: most of a page part way through a slide is past them.
func (s shifted) Clipped() Rect {
	c := ClipOf(s.to)
	if c.W <= 0 || c.H <= 0 {
		w, h := s.to.Size()
		c = Rect{W: w, H: h}
	}
	return Rect{X: c.X - s.dx, Y: c.Y - s.dy, W: c.W, H: c.H}
}
