package boot

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/reveal"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// The panel is 1200x1920 and is stood either way up, so both are the real thing rather than a
// convenient square.
var sizes = []struct {
	name string
	w, h int
}{
	{"landscape", 1920, 1200},
	{"portrait", 1200, 1920},
}

func coming() []component.Progress {
	return []component.Progress{
		{Name: "wifi", Doing: "looking for the network"},
		{Name: "dhcp", Doing: "asking for an address", Done: true},
	}
}

// The mark goes beside the list on a wide picture and above it on a tall one, and the two never
// overlap: the list is drawn over whatever the logo left behind.
func TestSplitDividesTheLongEdge(t *testing.T) {
	for _, s := range sizes {
		logo, list := reveal.Split(s.w, s.h)

		if logo.W <= 0 || logo.H <= 0 || list.W <= 0 || list.H <= 0 {
			t.Fatalf("%s: split gave an empty area: logo %+v list %+v", s.name, logo, list)
		}

		if s.w >= s.h {
			if logo.X+logo.W != list.X {
				t.Errorf("%s: the list starts at %d, want the logo's right edge %d",
					s.name, list.X, logo.X+logo.W)
			}
			if logo.H != s.h || list.H != s.h {
				t.Errorf("%s: a side by side split should use the full height", s.name)
			}
			continue
		}

		if logo.Y+logo.H != list.Y {
			t.Errorf("%s: the list starts at %d, want the logo's bottom edge %d",
				s.name, list.Y, logo.Y+logo.H)
		}
		if logo.W != s.w || list.W != s.w {
			t.Errorf("%s: a stacked split should use the full width", s.name)
		}
	}
}

// Everything drawn has to land on the picture. The panel is written pixel by pixel, so anything
// off the edge is a write past the buffer rather than something clipped.
func TestDrawBootFitsBothWaysUp(t *testing.T) {
	for _, s := range sizes {
		img := ui.NewImage(s.w, s.h, theme.Brand().Surface)

		if err := drawBoot(img, coming()); err != nil {
			t.Fatalf("%s: drawBoot: %v", s.name, err)
		}
	}
}

// The list is the only account anyone gets of what happened at start-up, so it has to actually
// appear rather than leaving the logo on its own.
func TestDrawBootWritesTheList(t *testing.T) {
	plain := ui.NewImage(1920, 1200, theme.Brand().Surface)
	if err := drawLogo(plain); err != nil {
		t.Fatalf("drawLogo: %v", err)
	}

	listed := ui.NewImage(1920, 1200, theme.Brand().Surface)
	if err := drawBoot(listed, coming()); err != nil {
		t.Fatalf("drawBoot: %v", err)
	}

	// Only in the half the list was given, so a differently placed logo is not what this sees.
	_, list := reveal.Split(1920, 1200)
	if same(plain, listed, list) {
		t.Error("the list area is identical with and without a list to draw")
	}
}

// A component that is up and one that is not have to look different, or the screen says nothing.
func TestDoneAndWaitingLookDifferent(t *testing.T) {
	waiting := ui.NewImage(1920, 1200, theme.Brand().Surface)
	if err := drawBoot(
		waiting,
		[]component.Progress{{Name: "wifi", Doing: "looking"}},
	); err != nil {
		t.Fatalf("drawBoot: %v", err)
	}

	done := ui.NewImage(1920, 1200, theme.Brand().Surface)
	if err := drawBoot(
		done,
		[]component.Progress{{Name: "wifi", Doing: "looking", Done: true}},
	); err != nil {
		t.Fatalf("drawBoot: %v", err)
	}

	_, list := reveal.Split(1920, 1200)
	if same(waiting, done, list) {
		t.Error("a component that is up is drawn the same as one that is not")
	}
}

// Nothing to wait for is the moment before the screen is handed over, and it still has to draw.
func TestDrawBootWithNothingWaiting(t *testing.T) {
	img := ui.NewImage(1920, 1200, theme.Brand().Surface)

	if err := drawBoot(img, nil); err != nil {
		t.Fatalf("drawBoot: %v", err)
	}
}

// Every full-screen screen carries the build, so which one a device is showing does not decide
// whether you can tell what it is running.
func TestEveryScreenShowsTheVersion(t *testing.T) {
	for _, tc := range []struct {
		name string
		draw func(ui.Surface) error
	}{
		{"logo", drawLogo},
		{"progress", func(s ui.Surface) error { return drawBoot(s, coming()) }},
		{"leaving", drawLeaving},
	} {
		plain := ui.NewImage(1920, 1200, theme.Brand().Surface)
		ui.Fill(plain, theme.Brand().Surface)

		img := ui.NewImage(1920, 1200, theme.Brand().Surface)
		if err := tc.draw(img); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		// The bottom right corner, which is the only thing drawn there.
		corner := ui.Rect{X: 1920 * 3 / 4, Y: 1200 * 3 / 4, W: 1920 / 4, H: 1200 / 4}
		if same(plain, img, corner) {
			t.Errorf("%s: nothing is drawn in the corner the build goes in", tc.name)
		}
	}
}

// It stays in its corner. A build string is long, and one that ran off the panel or across the
// list would be worse than not showing it.
func TestVersionStaysInItsCorner(t *testing.T) {
	for _, s := range sizes {
		img := ui.NewImage(s.w, s.h, theme.Brand().Surface)
		blank := ui.NewImage(s.w, s.h, theme.Brand().Surface)

		drawVersion(img, theme.Brand())

		// Nothing outside the bottom right eighth of the picture.
		kept := ui.Rect{X: s.w / 2, Y: s.h * 7 / 8, W: s.w / 2, H: s.h / 8}
		for y := range s.h {
			for x := range s.w {
				if kept.Contains(x, y) {
					continue
				}
				if img.At(x, y) != blank.At(x, y) {
					t.Fatalf("%s: the build is drawn at %d,%d, outside its corner", s.name, x, y)
				}
			}
		}
	}
}

// same reports whether two pictures agree everywhere in an area.
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
