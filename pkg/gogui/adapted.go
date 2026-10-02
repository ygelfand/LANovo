// Adapted from go-gui (MIT): gui/backend/internal/gpu and gui/backend/internal/glyphconv; see NOTICE.
package gogui

import (
	"math"

	"github.com/go-gui-org/go-glyph"
	"github.com/go-gui-org/go-gui/gui"
)

type vertex struct {
	X, Y, Z    float32
	U, V       float32
	R, G, B, A float32
}

func normColor(r, g, b, a uint8) (rf, gf, bf, af float32) {
	return float32(r) / 255, float32(g) / 255, float32(b) / 255, float32(a) / 255
}

func badFloat(f float32) bool { return math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) }

func packParams(radius, thickness float32) float32 {
	if badFloat(radius) {
		radius = 0
	}
	if badFloat(thickness) {
		thickness = 0
	}
	return float32(math.Floor(float64(radius)*4))*4096 + float32(math.Floor(float64(thickness)*4))
}

func buildQuad(x, y, w, h float32, c gui.Color, radius, thickness float32) [4]vertex {
	z := packParams(radius, thickness)
	r, g, b, a := normColor(c.R, c.G, c.B, c.A)
	return [4]vertex{
		{x, y, z, -1, -1, r, g, b, a},
		{x + w, y, z, 1, -1, r, g, b, a},
		{x + w, y + h, z, 1, 1, r, g, b, a},
		{x, y + h, z, -1, 1, r, g, b, a},
	}
}

const clipCoordLimit = float64(1 << 24)

func clampClipCoord(v float64) int32 {
	switch {
	case math.IsNaN(v):
		return 0
	case v > clipCoordLimit:
		return int32(clipCoordLimit)
	case v < -clipCoordLimit:
		return int32(-clipCoordLimit)
	}
	return int32(v)
}

func clipRect(x, y, w, h float32) (cx, cy, cw, ch int32) {
	x0 := clampClipCoord(math.Floor(float64(x)))
	y0 := clampClipCoord(math.Floor(float64(y)))
	x1 := clampClipCoord(math.Ceil(float64(x) + float64(w)))
	y1 := clampClipCoord(math.Ceil(float64(y) + float64(h)))
	cw, ch = max(x1-x0, 0), max(y1-y0, 0)
	if w <= 0 {
		cw = 0
	}
	if h <= 0 {
		ch = 0
	}
	return x0, y0, cw, ch
}

func textConfigFromRender(r *gui.RenderCmd) glyph.TextConfig {
	var cfg glyph.TextConfig
	if r.TextStylePtr != nil {
		cfg = styleToGlyphConfig(*r.TextStylePtr)
		cfg.Gradient = r.TextGradient
	} else {
		cfg = glyph.TextConfig{
			Style: glyph.TextStyle{
				FontName: r.FontName,
				Size:     r.FontSize,
				Color:    glyph.Color{R: r.Color.R, G: r.Color.G, B: r.Color.B, A: r.Color.A},
			},
			Block: glyph.DefaultBlockStyle(),
		}
	}
	if r.W > 0 {
		cfg.Block.Wrap = glyph.WrapWord
		cfg.Block.Width = r.W
	}
	return cfg
}

func styleToGlyphConfig(s gui.TextStyle) glyph.TextConfig {
	align := glyph.AlignLeft
	switch s.Align {
	case gui.TextAlignCenter:
		align = glyph.AlignCenter
	case gui.TextAlignRight:
		align = glyph.AlignRight
	}
	return glyph.TextConfig{
		Style: glyph.TextStyle{
			FontName:           s.Family,
			Size:               s.Size,
			Color:              glyph.Color{R: s.Color.R, G: s.Color.G, B: s.Color.B, A: s.Color.A},
			BgColor:            glyph.Color{R: s.BgColor.R, G: s.BgColor.G, B: s.BgColor.B, A: s.BgColor.A},
			Typeface:           s.Typeface,
			Underline:          s.Underline,
			Strikethrough:      s.Strikethrough,
			LetterSpacing:      s.LetterSpacing,
			EmojiBoxWidth:      s.EmojiBoxWidth,
			CellWidth:          s.CellWidth,
			CellHeight:         s.CellHeight,
			NoBuiltinBoxGlyphs: s.NoBuiltinBoxGlyphs,
			StrokeWidth:        s.StrokeWidth,
			StrokeColor:        glyph.Color{R: s.StrokeColor.R, G: s.StrokeColor.G, B: s.StrokeColor.B, A: s.StrokeColor.A},
			Features:           s.Features,
		},
		Block: glyph.BlockStyle{
			Align:       align,
			Wrap:        glyph.WrapWord,
			Width:       -1,
			LineSpacing: s.LineSpacing,
		},
		Gradient: s.Gradient,
	}
}

func identityTM() [16]float32 { return [16]float32{0: 1, 5: 1, 10: 1, 15: 1} }

const (
	gradientStopSlots = 12
	stopsInTM         = 4
)

func packGradientUniforms(gdef *gui.GradientDef, stops []gui.GradientStop, w, h float32) (tm, tm2 [16]float32) {
	n := min(len(stops), gradientStopSlots)
	for i := range n {
		dst, slot := &tm, i
		if i >= stopsInTM {
			dst, slot = &tm2, i-stopsInTM
		}
		dst[slot*2] = gui.PackRGB(stops[i].Color)
		dst[slot*2+1] = gui.PackAlphaPos(stops[i].Color, stops[i].Pos)
	}
	if gdef != nil && gdef.Type == gui.GradientRadial {
		tm[2*4+3] = max(w/2, h/2)
		tm[3*4+2] = 1
	} else {
		dx, dy := gui.GradientDir(gdef, w, h)
		tm[2*4+2] = dx
		tm[2*4+3] = dy
		tm[3*4+2] = 0
	}
	tm[3*4+0] = w / 2
	tm[3*4+1] = h / 2
	tm[3*4+3] = float32(n)
	return tm, tm2
}
