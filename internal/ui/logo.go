package ui

import (
	"bytes"
	_ "embed"
	"image"
	_ "image/png"
	"sync"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// The mark, one for each kind of background. Embedded so they cannot go missing, and here rather
// than with the framebuffer because this is something the device draws, not something the panel
// knows about.
//
// Two drawings rather than one and a rule: the dark one is not the light one inverted. Scaled from
// the artwork in assets by make logo.
//
//go:embed logo_light.png
var logoPNG []byte

//go:embed logo_dark.png
var nightPNG []byte

var (
	logoOnce sync.Once
	logo     image.Image

	nightOnce sync.Once
	night     image.Image
)

// Logo is the mark, decoded once.
func Logo() image.Image { return decode(&logoOnce, &logo, logoPNG) }

// Night is the mark for a dark background.
func Night() image.Image { return decode(&nightOnce, &night, nightPNG) }

var (
	nightSurfaceOnce sync.Once
	nightSurface     *Image
)

func NightSurface() *Image {
	nightSurfaceOnce.Do(func() { nightSurface, _ = Decode(nightPNG) })
	return nightSurface
}

func decode(once *sync.Once, into *image.Image, raw []byte) image.Image {
	once.Do(func() {
		img, _, err := image.Decode(bytes.NewReader(raw))
		if err == nil {
			*into = img
		}
	})
	return *into
}

// MarkWidth is how wide the mark comes out at a height.
//
// For a caller placing it: DrawLogo centers the mark in whatever box it is given, so a box that is
// not the mark's shape leaves it floating, and a mark meant for a corner ends up short of it.
func MarkWidth(h int) int {
	img := Logo()
	if img == nil {
		return h
	}

	b := img.Bounds()
	if b.Dy() == 0 {
		return h
	}
	return b.Dx() * h / b.Dy()
}

// DrawLogo paints the mark as large as fits in an area, keeping its aspect, centered.
//
// on is what it is composited against: the mark has soft edges, and blending them into the wrong
// color leaves a halo of whatever it was drawn for last. It is also what decides which way round
// the mark is drawn, so a caller says where the mark goes and what is behind it, and never which
// of the two versions it wanted — there is no way for those to disagree if it is never asked.
func DrawLogo(s Surface, r Rect, on theme.Color) {
	img := Logo()
	if theme.Dark(on) {
		img = Night()
	}
	if img == nil {
		return
	}

	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 || r.W <= 0 || r.H <= 0 {
		return
	}

	// Scale by whichever edge runs out first.
	w, h := b.Dx(), b.Dy()
	if w*r.H > h*r.W {
		h = h * r.W / w
		w = r.W
	} else {
		w = w * r.H / h
		h = r.H
	}

	at := Rect{X: r.X + (r.W-w)/2, Y: r.Y + (r.H-h)/2, W: w, H: h}
	drawScaled(s, img, at, on)
}

// drawScaled paints img into a box, nearest-neighbor, composited over a background color.
func drawScaled(s Surface, img image.Image, at Rect, on theme.Color) {
	b := img.Bounds()

	for y := range at.H {
		sy := b.Min.Y + y*b.Dy()/at.H
		for x := range at.W {
			sx := b.Min.X + x*b.Dx()/at.W

			// RGBA() is alpha-premultiplied, which is what source-over wants:
			// out = src + dst*(1-alpha).
			sr, sg, sb, sa := img.At(sx, sy).RGBA()
			if sa == 0 {
				continue
			}

			c := theme.Color{R: byte(sr >> 8), G: byte(sg >> 8), B: byte(sb >> 8)}
			if sa < 0xffff {
				inv := 0xffff - sa
				c = theme.Color{
					R: byte((sr + uint32(on.R)*0x101*inv/0xffff) >> 8),
					G: byte((sg + uint32(on.G)*0x101*inv/0xffff) >> 8),
					B: byte((sb + uint32(on.B)*0x101*inv/0xffff) >> 8),
				}
			}
			setClipped(s, at.X+x, at.Y+y, c)
		}
	}
}
