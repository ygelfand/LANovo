package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/exp/shiny/iconvg"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Icons that are SVG rather than IconVG.
//
// IconVG holds one channel of coverage and nothing else, which is why the tool that writes it
// refuses a stroked path, a path that fills nothing, or more than one color: none of them survive
// the encoding. The sets worth having — arcticons for the look, selfhst for the logos — are all
// three of those things.
//
// So the drawing is done from the SVG itself. An icon stays checked in as the file it was
// downloaded as, readable and swappable, and the rasterizer sorts out strokes, caps and joins.

// vgMagic is what an IconVG file starts with, which is how the two are told apart. Nothing else is
// guessed at: an icon is one format or the other by its first four bytes.
var vgMagic = []byte{0x89, 'I', 'V', 'G'}

// IsVG reports whether an icon is IconVG rather than SVG.
func IsVG(icon Icon) bool { return bytes.HasPrefix(icon, vgMagic) }

// Icons drawn in their own colors, which is a logo rather than a glyph.
//
// A separate cache from the coverage one, and deliberately so: what fills that cache is numerals,
// half a megabyte of alpha each, and holding the color behind every one of them would be four
// times the memory for something no numeral ever asks for. Both are counted against the same
// budget, so a page full of logos still cannot grow without limit.
var colorCache = map[iconKey]*image.RGBA{}

// inColor rasterizes an icon keeping its own colors, or nil if it will not draw.
func inColor(icon Icon, w, h int) *image.RGBA {
	at := iconKey{icon: string(icon), w: w, h: h}

	iconMu.Lock()
	defer iconMu.Unlock()

	if img, ok := colorCache[at]; ok {
		return img
	}

	img, err := raster(icon, w, h)
	if err != nil {
		// Remembered as nothing, so a broken icon is not re-rasterized every frame.
		colorCache[at] = nil
		return nil
	}

	if iconBytes+len(img.Pix) > iconBudget {
		evict(at)
	}
	colorCache[at] = img
	iconBytes += len(img.Pix)
	return img
}

// DrawColored paints an icon in the colors it was drawn with, fitted to a square in the middle of
// r and composited over on.
//
// For a logo, where the colors are the point and a single-color silhouette would not be
// recognisable. Everything else should use DrawIcon, which takes the color from the theme and
// costs a quarter of the memory.
func DrawColored(s Surface, icon Icon, r Rect, on theme.Color) {
	side := min(r.W, r.H)
	if side <= 0 {
		return
	}

	img := inColor(icon, side, side)
	if img == nil {
		return
	}
	drawScaled(s, img, Rect{X: r.X + (r.W-side)/2, Y: r.Y + (r.H-side)/2, W: side, H: side}, on)
}

// raster draws an icon of either format into a fresh image at a size.
//
// The one place the two are told apart. Everything above it works in coverage or in pixels and does
// not care which decoder produced them.
func raster(icon Icon, w, h int) (*image.RGBA, error) {
	if IsVG(icon) {
		return paintVG(icon, w, h)
	}
	return paint(icon, w, h)
}

// paintVG draws IconVG, whose palette is thrown away: only the coverage is kept, so the icon is
// drawn white on nothing and the alpha channel is the answer.
func paintVG(icon Icon, w, h int) (*image.RGBA, error) {
	out := image.NewRGBA(image.Rect(0, 0, w, h))

	var z iconvg.Rasterizer
	z.SetDstImage(out, out.Bounds(), draw.Src)

	opts := iconvg.DecodeOptions{
		Palette: &iconvg.Palette{0: color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}},
	}
	if err := iconvg.Decode(&z, icon, &opts); err != nil {
		return nil, fmt.Errorf("ui: decoding an iconvg icon: %w", err)
	}
	return out, nil
}

// paint rasterizes an SVG into a fresh image at a size.
//
// The icon's own colors are kept rather than forced to white: oksvg does not expose the fields to
// change them, and there is no need. A mask takes the alpha, which is coverage whatever color the
// shapes are, and a logo wants the colors anyway.
func paint(icon Icon, w, h int) (*image.RGBA, error) {
	svg, err := oksvg.ReadIconStream(bytes.NewReader(icon))
	if err != nil {
		return nil, fmt.Errorf("ui: reading an svg icon: %w", err)
	}
	if svg.ViewBox.W == 0 || svg.ViewBox.H == 0 {
		return nil, fmt.Errorf("ui: an svg icon with no view box")
	}

	// Fitted to a square inside the target and centred, so a wide icon is not stretched. The
	// caller has already decided the box; this decides the drawing inside it.
	side := min(float64(w), float64(h))
	scale := min(side/svg.ViewBox.W, side/svg.ViewBox.H)

	dw, dh := svg.ViewBox.W*scale, svg.ViewBox.H*scale
	svg.SetTarget((float64(w)-dw)/2, (float64(h)-dh)/2, dw, dh)

	out := image.NewRGBA(image.Rect(0, 0, w, h))
	svg.Draw(rasterx.NewDasher(w, h, rasterx.NewScannerGV(w, h, out, out.Bounds())), 1)
	return out, nil
}
