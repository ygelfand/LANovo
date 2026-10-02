package ui

import (
	"image"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Within is a surface that only takes paint inside a rectangle.
//
// The same thing the driver does to the panel once damage has been declared, available over an
// image so a test can draw a narrowed repaint and check it against a full one. A declared rectangle
// is only right if those two agree.
//
// It reads back through whatever is underneath, which antialiasing needs, and reports the rectangle
// as its clip so drawing can skip work outside it rather than rasterising and discarding.
func Within(s Surface, to Rect) Surface { return within{under: s, to: to} }

type within struct {
	under Surface
	to    Rect
}

func (w within) Size() (int, int) { return w.under.Size() }
func (w within) Clipped() Rect    { return w.to }

func (w within) Set(x, y int, c theme.Color) {
	if !w.to.Contains(x, y) {
		return
	}
	w.under.Set(x, y, c)
}

func (w within) Over(x, y int, c theme.Color, a byte) {
	if w.to.Contains(x, y) {
		Over(w.under, x, y, c, a)
	}
}

func (w within) clip(r Rect) (Rect, bool) {
	x0, y0 := max(r.X, w.to.X), max(r.Y, w.to.Y)
	x1, y1 := min(r.X+r.W, w.to.X+w.to.W), min(r.Y+r.H, w.to.Y+w.to.H)
	if x1 <= x0 || y1 <= y0 {
		return Rect{}, false
	}
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}, true
}

func (w within) FillRect(r Rect, c theme.Color) {
	if r, ok := w.clip(r); ok {
		FillRect(w.under, r, c)
	}
}

func (w within) ClearRect(r Rect) {
	if r, ok := w.clip(r); ok {
		Clear(w.under, r)
	}
}

func (w within) Shade(r Rect, top, bottom byte) {
	c, ok := w.clip(r)
	if !ok {
		return
	}
	span := max(r.H-1, 1)
	at := func(y int) byte { return byte(int(top) + (int(bottom)-int(top))*(y-r.Y)/span) }
	Shade(w.under, c, at(c.Y), at(c.Y+c.H-1))
}

func (w within) DrawRGBA(x, y int, img *image.RGBA, scale int, clip Rect) {
	to := w.to
	if clip.W > 0 && clip.H > 0 {
		x0, y0 := max(to.X, clip.X), max(to.Y, clip.Y)
		x1, y1 := min(to.X+to.W, clip.X+clip.W), min(to.Y+to.H, clip.Y+clip.H)
		if x1 <= x0 || y1 <= y0 {
			return
		}
		to = Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
	}
	DrawRGBA(w.under, x, y, img, scale, to)
}

func (w within) At(x, y int) theme.Color {
	if r, ok := w.under.(Reader); ok {
		return r.At(x, y)
	}
	return theme.Color{}
}

// Changed is the smallest rectangle holding every pixel where two pictures differ, and is empty
// when they do not differ at all.
//
// What it is for: checking a damage rectangle. Declaring damage is how a repaint is made cheap, and
// declaring one too small leaves a stale strip of screen that nothing catches until somebody looks
// at the panel. The check that does catch it is this — draw the state before, draw the state after,
// and see that everything which moved is inside what was declared.
//
// Pictures of different sizes have nothing meaningful to compare, so the whole of the larger one is
// the answer: a caller that has changed the size has changed everything.
func Changed(a, b *Image) Rect {
	aw, ah := a.Size()
	bw, bh := b.Size()

	if aw != bw || ah != bh {
		return Rect{W: max(aw, bw), H: max(ah, bh)}
	}

	left, top := aw, ah
	right, bottom := -1, -1

	for y := range ah {
		for x := range aw {
			if a.At(x, y) == b.At(x, y) {
				continue
			}
			if x < left {
				left = x
			}
			if x > right {
				right = x
			}
			if y < top {
				top = y
			}
			if y > bottom {
				bottom = y
			}
		}
	}

	if right < 0 {
		return Rect{}
	}
	return Rect{X: left, Y: top, W: right - left + 1, H: bottom - top + 1}
}

// Union is the smallest rectangle holding both, ignoring an empty one: a part that is not drawn
// contributes nothing rather than dragging the rectangle to the origin.
func (r Rect) Union(o Rect) Rect {
	if o.W <= 0 || o.H <= 0 {
		return r
	}
	if r.W <= 0 || r.H <= 0 {
		return o
	}

	left := min(r.X, o.X)
	top := min(r.Y, o.Y)
	right := max(r.X+r.W, o.X+o.W)
	bottom := max(r.Y+r.H, o.Y+o.H)

	return Rect{X: left, Y: top, W: right - left, H: bottom - top}
}

// Holds reports whether r covers the whole of inner, which is what a damage rectangle has to do for
// the pixels that changed.
//
// An empty inner is held by anything: nothing changed, so there is nothing to cover.
func (r Rect) Holds(inner Rect) bool {
	if inner.W <= 0 || inner.H <= 0 {
		return true
	}
	if r.W <= 0 || r.H <= 0 {
		return false
	}
	return inner.X >= r.X && inner.Y >= r.Y &&
		inner.X+inner.W <= r.X+r.W && inner.Y+inner.H <= r.Y+r.H
}
