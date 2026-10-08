package dashboard

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

var screens = []struct {
	name string
	w, h int
}{
	{"landscape", 1920, 1200},
	{"portrait", 1200, 1920},
}

func TestEveryBoxIsOnTheScreen(t *testing.T) {
	for _, at := range config.Positions() {
		for _, size := range config.Sizes() {
			for _, s := range screens {
				box := Box(at, size, s.w, s.h)

				if box.X < 0 || box.Y < 0 {
					t.Errorf("%s %s %s: starts at %d,%d", at, size, s.name, box.X, box.Y)
				}
				if box.X+box.W > s.w || box.Y+box.H > s.h {
					t.Errorf("%s %s %s: runs to %d,%d past %dx%d",
						at, size, s.name, box.X+box.W, box.Y+box.H, s.w, s.h)
				}
				if box.W <= 0 || box.H <= 0 {
					t.Errorf("%s %s %s: is %dx%d", at, size, s.name, box.W, box.H)
				}
			}
		}
	}
}

func TestThePositionsAreInOrder(t *testing.T) {
	for _, size := range config.Sizes() {
		for _, s := range screens {
			mid := func(at config.Position) int {
				box := Box(at, size, s.w, s.h)
				return box.Y + box.H/2
			}

			top := mid(config.PositionTop)
			center := mid(config.PositionCenter)
			bottom := mid(config.PositionBottom)

			if top >= center || center >= bottom {
				t.Errorf("%s %s: the middles are top %d, center %d, bottom %d",
					size, s.name, top, center, bottom)
			}
		}
	}
}

func TestCenteredAndLargeIsTheWholeScreen(t *testing.T) {
	for _, s := range screens {
		box := Box(config.PositionCenter, config.SizeLarge, s.w, s.h)

		if box.X != 0 || box.Y != 0 || box.W != s.w || box.H != s.h {
			t.Errorf("%s: centered is %v, want the whole %dx%d", s.name, box, s.w, s.h)
		}
	}
}

func TestAnUnknownPositionIsCentered(t *testing.T) {
	got := Box(config.Position("sideways"), config.DefaultSize, 1200, 1920)

	if want := Box(config.PositionCenter, config.DefaultSize, 1200, 1920); got != want {
		t.Errorf("an unknown position is %v, want the centered %v", got, want)
	}
}
