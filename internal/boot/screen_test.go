package boot

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/display/ui"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

var sizes = []struct {
	name string
	w, h int
}{
	{"landscape", 1920, 1200},
	{"portrait", 1200, 1920},
}

func TestEveryScreenShowsTheVersion(t *testing.T) {
	for _, tc := range []struct {
		name string
		draw func(ui.Surface) error
	}{
		{"logo", drawLogo},
		{"leaving", drawLeaving},
	} {
		plain := ui.NewImage(1920, 1200, theme.Brand().Surface)
		ui.Fill(plain, theme.Brand().Surface)

		img := ui.NewImage(1920, 1200, theme.Brand().Surface)
		if err := tc.draw(img); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		corner := ui.Rect{X: 1920 * 3 / 4, Y: 1200 * 3 / 4, W: 1920 / 4, H: 1200 / 4}
		if same(plain, img, corner) {
			t.Errorf("%s: nothing is drawn in the corner the build goes in", tc.name)
		}
	}
}

func same(a, b *ui.Image, in ui.Rect) bool {
	for y := in.Y; y < in.Y+in.H; y++ {
		for x := in.X; x < in.X+in.W; x++ {
			if a.At(x, y) != b.At(x, y) {
				return false
			}
		}
	}
	return true
}
