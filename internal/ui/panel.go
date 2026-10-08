package ui

import (
	"image"

	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

type panel struct{ p *display.Panel }

func Of(p *display.Panel) Surface { return panel{p: p} }

func (s panel) Size() (w, h int) { return s.p.Width, s.p.Height }

func (s panel) Set(x, y int, c theme.Color) { s.p.Set(x, y, c.R, c.G, c.B) }

func (s panel) FillRect(r Rect, c theme.Color) {
	s.p.FillRect(r.X, r.Y, r.W, r.H, c.R, c.G, c.B)
}

func (s panel) Over(x, y int, c theme.Color, a byte) { s.p.Over(x, y, c.R, c.G, c.B, a) }

func (s panel) ClearRect(r Rect) { s.p.ClearRect(r.X, r.Y, r.W, r.H) }

func (s panel) Shade(r Rect, top, bottom byte) { s.p.Shade(r.X, r.Y, r.W, r.H, top, bottom) }

func (s panel) At(x, y int) theme.Color {
	r, g, b := s.p.At(x, y)
	return theme.Color{R: r, G: g, B: b}
}

func (s panel) DrawRGBA(x, y int, img *image.RGBA, scale int, clip Rect) {
	b := img.Bounds()
	s.p.DrawRGBA(x, y, img.Pix, img.Stride, b.Dx(), b.Dy(), scale,
		display.Rect{X: clip.X, Y: clip.Y, W: clip.W, H: clip.H})
}

func (s panel) Clipped() Rect {
	r := s.p.Clipped()
	return Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
}
