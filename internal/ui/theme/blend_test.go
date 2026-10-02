package theme

import "testing"

// Whether the float64 in Blend is what costs, which is the premise #83 is built on. The comparison
// is against fixed point over the same coverage values a glyph produces, since that is where the
// blending happens: one call per covered pixel, with the coverage as a byte.

// blendFixed is the same mix in integers. Kept in the test rather than the package: this is a
// measurement of whether the change is worth making, not the change.
func blendFixed(c, other Color, a byte) Color {
	switch a {
	case 0:
		return c
	case 255:
		return other
	}

	// Rounded, and over 255 rather than 256, so full coverage lands exactly on the other color
	// instead of one short of it.
	mix := func(x, y byte) byte {
		return byte((int(x)*(255-int(a)) + int(y)*int(a) + 127) / 255)
	}
	return Color{mix(c.R, other.R), mix(c.G, other.G), mix(c.B, other.B)}
}

// coverage is what a rasterized glyph hands over: mostly solid or empty, with edges in between.
func coverage() []byte {
	out := make([]byte, 4096)
	for i := range out {
		out[i] = byte(i * 251 % 256)
	}
	return out
}

var sink Color

func BenchmarkBlend(b *testing.B) {
	fg := Color{R: 0xf5, G: 0xf7, B: 0xfa}
	bg := Color{R: 0x11, G: 0x13, B: 0x18}
	cover := coverage()

	b.Run("float", func(b *testing.B) {
		var last Color
		for b.Loop() {
			for _, a := range cover {
				last = bg.Blend(fg, float64(a)/255)
			}
		}
		sink = last
	})

	b.Run("fixed", func(b *testing.B) {
		var last Color
		for b.Loop() {
			for _, a := range cover {
				last = blendFixed(bg, fg, a)
			}
		}
		sink = last
	})
}

// The two have to agree, or the comparison is between a blend and something else. One step of 255
// is allowed: the float rounds towards zero and the fixed point rounds to nearest, so they part by
// at most one.
func TestFixedAgreesWithFloat(t *testing.T) {
	fg := Color{R: 0xf5, G: 0x00, B: 0xfa}
	bg := Color{R: 0x11, G: 0xff, B: 0x18}

	for a := range 256 {
		want := bg.Blend(fg, float64(a)/255)
		got := blendFixed(bg, fg, byte(a))

		for _, at := range []struct {
			name     string
			got, wnt byte
		}{
			{"R", got.R, want.R}, {"G", got.G, want.G}, {"B", got.B, want.B},
		} {
			if d := int(at.got) - int(at.wnt); d < -1 || d > 1 {
				t.Errorf("coverage %d: %s is %d, float says %d", a, at.name, at.got, at.wnt)
			}
		}
	}
}

// The ends have to be exact, whatever the middle does: a fully covered pixel is the ink and an
// uncovered one is untouched. Anything else leaves a glyph sitting on a box a shade off the
// background.
func TestTheEndsAreExact(t *testing.T) {
	fg := Color{R: 0xf5, G: 0xf7, B: 0xfa}
	bg := Color{R: 0x11, G: 0x13, B: 0x18}

	if got := blendFixed(bg, fg, 0); got != bg {
		t.Errorf("no coverage gave %v, want the background %v", got, bg)
	}
	if got := blendFixed(bg, fg, 255); got != fg {
		t.Errorf("full coverage gave %v, want the ink %v", got, fg)
	}
}
