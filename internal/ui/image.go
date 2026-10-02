package ui

import (
	"image"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Image is a surface in memory, for drawing something before it goes to the panel and for
// checking what was drawn without one.
type Image struct {
	w, h int
	px   []theme.Color
}

// NewImage is a surface of this size, filled with a color.
func NewImage(w, h int, c theme.Color) *Image {
	img := &Image{w: w, h: h, px: make([]theme.Color, w*h)}
	img.FillRect(Rect{W: w, H: h}, c)
	return img
}

func (i *Image) Size() (w, h int) { return i.w, i.h }

func (i *Image) Set(x, y int, c theme.Color) {
	if x < 0 || y < 0 || x >= i.w || y >= i.h {
		return
	}
	i.px[y*i.w+x] = c
}

// At is the color of one pixel. A point outside the image is the zero color, since nothing was
// drawn there.
func (i *Image) At(x, y int) theme.Color {
	if x < 0 || y < 0 || x >= i.w || y >= i.h {
		return theme.Color{}
	}
	return i.px[y*i.w+x]
}

// RGBADrawer is a surface that takes a whole RGBA image at once, each pixel scale times across and
// down, painting only inside clip.
type RGBADrawer interface {
	DrawRGBA(x, y int, img *image.RGBA, scale int, clip Rect)
}

// DrawRGBA paints img onto s through its RGBADrawer when it has one, and a pixel at a time when
// it does not.
func DrawRGBA(s Surface, x, y int, img *image.RGBA, scale int, clip Rect) {
	if d, ok := s.(RGBADrawer); ok {
		d.DrawRGBA(x, y, img, scale, clip)
		return
	}
	scale = max(scale, 1)
	b := img.Bounds()
	w, h := s.Size()
	x0, y0 := max(x, 0), max(y, 0)
	x1, y1 := min(x+b.Dx()*scale, w), min(y+b.Dy()*scale, h)
	if clip.W > 0 && clip.H > 0 {
		x0, y0 = max(x0, clip.X), max(y0, clip.Y)
		x1, y1 = min(x1, clip.X+clip.W), min(y1, clip.Y+clip.H)
	}
	for py := y0; py < y1; py++ {
		src := img.Pix[((py-y)/scale)*img.Stride:]
		for px := x0; px < x1; px++ {
			i := ((px - x) / scale) * 4
			s.Set(px, py, theme.Color{R: src[i], G: src[i+1], B: src[i+2]})
		}
	}
}

func (i *Image) DrawRGBA(x, y int, img *image.RGBA, scale int, clip Rect) {
	scale = max(scale, 1)
	b := img.Bounds()
	x0, y0 := max(x, 0), max(y, 0)
	x1, y1 := min(x+b.Dx()*scale, i.w), min(y+b.Dy()*scale, i.h)
	if clip.W > 0 && clip.H > 0 {
		x0, y0 = max(x0, clip.X), max(y0, clip.Y)
		x1, y1 = min(x1, clip.X+clip.W), min(y1, clip.Y+clip.H)
	}
	if x1 <= x0 || y1 <= y0 {
		return
	}
	sx0, phase0 := (x0-x)/scale, (x0-x)%scale
	sy, phy := (y0-y)/scale, (y0-y)%scale
	built := -1
	for py := y0; py < y1; py++ {
		dst := i.px[py*i.w+x0 : py*i.w+x1]
		if sy == built {
			copy(dst, i.px[(py-1)*i.w+x0:(py-1)*i.w+x1])
		} else {
			src := img.Pix[sy*img.Stride:]
			s, ph := sx0*4, phase0
			for k := range dst {
				dst[k] = theme.Color{R: src[s], G: src[s+1], B: src[s+2]}
				if ph++; ph == scale {
					ph, s = 0, s+4
				}
			}
			built = sy
		}
		if phy++; phy == scale {
			phy, sy = 0, sy+1
		}
	}
}

// Draw copies an image onto a surface with its top-left corner at x, y.
//
// What lands outside is worked out once rather than per pixel: a full screen is two million of
// them, and asking the surface its size that many times costs more than the copy.
func Draw(s Surface, img *Image, x, y int) {
	w, h := s.Size()

	x0, y0 := max(-x, 0), max(-y, 0)
	x1, y1 := min(img.w, w-x), min(img.h, h-y)

	if clip := ClipOf(s); clip.W > 0 && clip.H > 0 {
		x0, y0 = max(x0, clip.X-x), max(y0, clip.Y-y)
		x1, y1 = min(x1, clip.X+clip.W-x), min(y1, clip.Y+clip.H-y)
	}
	if x1 <= x0 || y1 <= y0 {
		return
	}

	for iy := y0; iy < y1; iy++ {
		row := img.px[iy*img.w+x0 : iy*img.w+x1]
		for i, c := range row {
			s.Set(x+x0+i, y+iy, c)
		}
	}
}

// DrawScaled copies an image to fill a rectangle, cropping to a square of it rather than squashing:
// album art is square and a panel's box for it may not be.
//
// Nearest neighbor. It runs when a screen is drawn, not per frame.
func DrawScaled(s Surface, img *Image, r Rect) {
	if r.W <= 0 || r.H <= 0 || img.w <= 0 || img.h <= 0 {
		return
	}

	// The largest centered piece of the source with the destination's proportions.
	sw, sh := img.w, img.h
	if sw*r.H > sh*r.W {
		sw = sh * r.W / r.H
	} else {
		sh = sw * r.H / r.W
	}
	ox, oy := (img.w-sw)/2, (img.h-sh)/2

	for y := range r.H {
		sy := oy + y*sh/r.H
		for x := range r.W {
			setClipped(s, r.X+x, r.Y+y, img.At(ox+x*sw/r.W, sy))
		}
	}
}

// FillRect paints whole rows of the pixel slice, which is what makes the drawing benchmarks
// measure the drawing rather than the bounds checks.
//
// One row written a pixel at a time, then copied into the others. A pixel is three bytes, so
// assigning one cannot be a word-sized store and a loop over every pixel moves the screen at a
// fraction of what the memory will take; copy is memmove, which moves words and does not care that
// three does not divide eight.
func (i *Image) FillRect(r Rect, c theme.Color) {
	x0, y0 := max(r.X, 0), max(r.Y, 0)
	x1, y1 := min(r.X+r.W, i.w), min(r.Y+r.H, i.h)
	if x1 <= x0 || y1 <= y0 {
		return
	}

	first := i.px[y0*i.w+x0 : y0*i.w+x1]
	for x := range first {
		first[x] = c
	}

	for y := y0 + 1; y < y1; y++ {
		copy(i.px[y*i.w+x0:y*i.w+x1], first)
	}
}
