package numeral

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/display/theme"
	"github.com/ygelfand/libcountertop/pkg/display/ui"
)

func TestEverySetStaysInsideTheBoxItIsGiven(t *testing.T) {
	palette := theme.Default()

	runs := []string{
		"12:34",
		"00:00",
		"9:05",
		":30",
		"12:",
		":",
		"1",
		"88:88",
		"11:11",
	}

	boxes := []ui.Rect{
		{W: 600, H: 200},
		{W: 300, H: 300},
		{W: 900, H: 120},
		{W: 61, H: 41},
		{W: 5, H: 5},
	}

	const pad = 16

	for _, set := range Sets {
		for _, run := range runs {
			for _, shape := range boxes {
				at := ui.Rect{X: pad, Y: pad, W: shape.W, H: shape.H}
				w, h := at.X+at.W+pad, at.Y+at.H+pad

				blank := ui.NewImage(w, h, palette.Background)
				img := ui.NewImage(w, h, palette.Background)

				set.Draw(img, run, at, palette.Text, palette.Background)

				if painted := ui.Changed(blank, img); !at.Holds(painted) {
					t.Errorf("%s drawing %q in %dx%d painted %+v, outside %+v",
						set.Name, run, shape.W, shape.H, painted, at)
				}
			}
		}
	}
}

func TestEverySetsDigitsStayInsideTheirCell(t *testing.T) {
	palette := theme.Default()

	const pad = 16
	at := ui.Rect{X: pad, Y: pad, W: 120, H: 200}
	w, h := at.X+at.W+pad, at.Y+at.H+pad

	for _, set := range Sets {
		for d := range 10 {
			blank := ui.NewImage(w, h, palette.Background)
			img := ui.NewImage(w, h, palette.Background)

			set.Digit(img, d, at, palette.Text, palette.Background)

			if painted := ui.Changed(blank, img); !at.Holds(painted) {
				t.Errorf("%s digit %d painted %+v, outside %+v", set.Name, d, painted, at)
			}
		}
	}
}

func TestWhatMeasureSaysHoldsWhatDrawPaints(t *testing.T) {
	palette := theme.Default()

	const pad = 16

	for _, set := range Sets {
		for _, run := range []string{"12:34", "00:00", "9:05", "88:88"} {
			for _, tall := range []int{40, 120, 200} {
				mw, mh := set.Measure(run, tall)

				at := ui.Rect{X: pad, Y: pad, W: mw, H: mh}
				w, h := at.X+at.W+pad, at.Y+at.H+pad

				blank := ui.NewImage(w, h, palette.Background)
				img := ui.NewImage(w, h, palette.Background)

				set.Draw(img, run, at, palette.Text, palette.Background)

				if painted := ui.Changed(blank, img); !at.Holds(painted) {
					t.Errorf("%s %q at %d tall measured %dx%d but painted %+v",
						set.Name, run, tall, mw, mh, painted)
				}
			}
		}
	}
}
