package ui

import (
	"fmt"
	"image"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Weight is how heavy the lettering is. Three is enough for a screen read across a room: one for
// body, one for labels, one for the thing being looked at.
type Weight int

const (
	Regular Weight = iota
	Medium
	Bold
)

// The Go fonts, which ship in the module rather than as an asset to install.
var faces = map[Weight][]byte{
	Regular: goregular.TTF,
	Medium:  gomedium.TTF,
	Bold:    gobold.TTF,
}

// Font is lettering at one size and weight.
type Font struct {
	face   font.Face
	height int

	ascent  int
	descent int

	// The face rasterizes into buffers it reuses and is not safe for two callers at once, so
	// every call into it is held here, and so is the cache those calls fill.
	mu     sync.Mutex
	glyphs map[rune]glyph
}

// glyph is one rune already rasterized at this font's size.
//
// Turning an outline into coverage is most of what drawing a string costs, and a screen redraws
// the same characters over and over: a clock face has ten of them, and a value being dragged is
// the same digits in a different order. Keeping the coverage turns every repeat into a copy.
type glyph struct {
	// cover is the rune's coverage, or nil for one that draws nothing, such as a space.
	cover *image.Alpha

	// at is the top-left of cover, relative to the pen on the baseline.
	at image.Point

	advance fixed.Int26_6
}

var (
	mu     sync.Mutex
	cached = map[key]*Font{}
	parsed = map[Weight]*opentype.Font{}
)

type key struct {
	weight Weight
	size   int
}

// Load is a font at a size in pixels. Faces are shared: a screen redrawing every second must not
// rasterize the same font again each time.
func Load(weight Weight, size int) (*Font, error) {
	mu.Lock()
	defer mu.Unlock()

	at := key{weight: weight, size: size}
	if f, ok := cached[at]; ok {
		return f, nil
	}

	ttf, ok := faces[weight]
	if !ok {
		return nil, fmt.Errorf("ui: no font for weight %d", weight)
	}

	if parsed[weight] == nil {
		p, err := opentype.Parse(ttf)
		if err != nil {
			return nil, fmt.Errorf("ui: parsing the font: %w", err)
		}
		parsed[weight] = p
	}

	face, err := opentype.NewFace(parsed[weight], &opentype.FaceOptions{
		Size: float64(size),
		DPI:  72, // a point is a pixel, so a size is the size it is asked for
	})
	if err != nil {
		return nil, fmt.Errorf("ui: sizing the font: %w", err)
	}

	metrics := face.Metrics()
	f := &Font{
		face:    face,
		height:  size,
		ascent:  metrics.Ascent.Round(),
		descent: metrics.Descent.Round(),
		glyphs:  map[rune]glyph{},
	}

	cached[at] = f
	return f, nil
}

// MustLoad is Load for a font that is compiled in, where a failure is a build problem rather than
// something a device can recover from.
func MustLoad(weight Weight, size int) *Font {
	f, err := Load(weight, size)
	if err != nil {
		panic(err)
	}
	return f
}

// Measure is how wide and tall a string will be.
func (f *Font) Measure(s string) (w, h int) {
	return f.width(s).Round(), f.ascent + f.descent
}

// Ascent is how far the lettering rises above its baseline, which is what a caller needs to place
// text by its top rather than its baseline.
func (f *Font) Ascent() int { return f.ascent }

// Descent is how far it drops below. Together with Ascent it is the line box, which is taller than
// the lettering in it: a caller stacking lines of digits wants the difference, because digits use
// none of the descent and a gap sized on the box reads as a gap somebody left by accident.
func (f *Font) Descent() int { return f.descent }

// width walks a string the same way drawing it does, so what is measured is where the glyphs land.
func (f *Font) width(s string) fixed.Int26_6 {
	var advance fixed.Int26_6

	prev := rune(-1)
	for _, r := range s {
		if prev >= 0 {
			advance += f.kern(prev, r)
		}
		advance += f.glyph(r).advance
		prev = r
	}
	return advance
}

// glyph is a rune's coverage, rasterized the first time it is asked for and kept afterwards.
//
// The coverage is copied out of the face. The face rasterizes into a buffer it reuses, so what it
// hands back is only valid until the next glyph is asked for, and keeping that would leave every
// cached rune showing whichever one was drawn last.
func (f *Font) glyph(r rune) glyph {
	f.mu.Lock()
	defer f.mu.Unlock()

	if g, ok := f.glyphs[r]; ok {
		return g
	}

	var g glyph
	area, src, from, advance, ok := f.face.Glyph(fixed.P(0, 0), r)
	g.advance = advance

	if ok && !area.Empty() {
		g.at = area.Min
		g.cover = image.NewAlpha(image.Rect(0, 0, area.Dx(), area.Dy()))

		if a, isAlpha := src.(*image.Alpha); isAlpha {
			for y := range area.Dy() {
				at := a.PixOffset(from.X, from.Y+y)
				copy(g.cover.Pix[y*g.cover.Stride:][:area.Dx()], a.Pix[at:at+area.Dx()])
			}
		} else {
			for y := range area.Dy() {
				for x := range area.Dx() {
					_, _, _, alpha := src.At(from.X+x, from.Y+y).RGBA()
					g.cover.Pix[y*g.cover.Stride+x] = uint8(alpha >> 8)
				}
			}
		}
	}

	f.glyphs[r] = g
	return g
}

// kern is the spacing the face wants between two runes.
func (f *Font) kern(prev, r rune) fixed.Int26_6 {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.face.Kern(prev, r)
}

// DrawText writes a string with its top-left corner at x, y.
//
// The color is mixed with the background it is drawn on rather than read back off the surface:
// the panel is a framebuffer that is written, not sampled, and text is always drawn onto
// something whose color is already known.
func DrawText(dst Surface, f *Font, x, y int, fg, bg theme.Color, s string) {
	clip := ClipOf(dst)
	if !clip.Overlaps(Rect{X: x, Y: y, W: f.width(s).Round(), H: f.ascent + f.descent}) {
		return
	}

	// The pen stays fractional and is only rounded to place each glyph, so the spacing a face asks
	// for is kept even though the coverage is cached at whole pixels.
	pen := fixed.I(x)
	base := y + f.ascent

	prev := rune(-1)
	for _, r := range s {
		if prev >= 0 {
			pen += f.kern(prev, r)
		}
		prev = r

		g := f.glyph(r)
		if g.cover != nil {
			blit(dst, g, pen.Round()+g.at.X, base+g.at.Y, fg, bg, clip)
		}
		pen += g.advance
	}
}

// blit puts one glyph's coverage down in a color.
func blit(dst Surface, g glyph, x, y int, fg, bg theme.Color, clip Rect) {
	b := g.cover.Bounds()
	if !clip.Overlaps(Rect{X: x, Y: y, W: b.Dx(), H: b.Dy()}) {
		return
	}

	for iy := range b.Dy() {
		row := g.cover.Pix[iy*g.cover.Stride:]

		for ix := range b.Dx() {
			switch a := row[ix]; a {
			case 0:
				continue
			case 0xff:
				setClipped(dst, x+ix, y+iy, fg)
			default:
				setClipped(dst, x+ix, y+iy, bg.Mix(fg, a))
			}
		}
	}
}

// DrawTextIn writes a string centered in an area.
func DrawTextIn(dst Surface, f *Font, r Rect, fg, bg theme.Color, s string) {
	w, h := f.Measure(s)
	DrawText(dst, f, r.X+(r.W-w)/2, r.Y+(r.H-h)/2, fg, bg, s)
}

// DrawTextRight writes a string with its right edge at the right of an area, vertically centered,
// which is what a column of times or values wants.
func DrawTextRight(dst Surface, f *Font, r Rect, fg, bg theme.Color, s string) {
	w, h := f.Measure(s)
	DrawText(dst, f, r.X+r.W-w, r.Y+(r.H-h)/2, fg, bg, s)
}
