package face

import (
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// every face there is, so a new one is covered by being registered rather than by being remembered.
func every() []config.Face {
	out := make([]config.Face, 0, len(registered))
	for name := range registered {
		out = append(out, name)
	}
	return out
}

// A face is handed a box and draws in it. Nothing checked that until now.
//
// It matters beyond tidiness: the box is what the dashboard clips a narrowed repaint to, and what
// it will hand out once it shares the screen with anything else (#77). A face that paints outside
// leaves a mark that whatever owns that space will not paint over.
func TestEveryFaceStaysInsideTheBoxItIsGiven(t *testing.T) {
	palette := theme.Default()

	// A box with room around it on all four sides, so an overrun in any direction lands somewhere
	// it can be seen rather than off the edge of the image.
	const pad = 60
	w, h := 1200, 1920
	in := ui.Rect{X: pad, Y: pad, W: w - 2*pad, H: h - 2*pad}

	readings := []Reading{
		Read(time.Date(2026, 9, 21, 14, 37, 0, 0, time.UTC), true),
		Read(time.Date(2026, 9, 21, 9, 5, 0, 0, time.UTC), false),
		Read(time.Date(2026, 9, 21, 23, 59, 0, 0, time.UTC), false),
		Read(time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC), true).Undated(),
	}

	for _, name := range every() {
		for _, r := range readings {
			blank := ui.NewImage(w, h, palette.Background)
			img := ui.NewImage(w, h, palette.Background)
			ui.Fill(img, palette.Background)

			Of(name).Draw(img, in, r, palette)

			if painted := ui.Changed(blank, img); !in.Holds(painted) {
				t.Errorf("%s drawing %q%s painted %+v, outside the box %+v",
					name, r.Time, r.Suffix, painted, in)
			}
		}
	}
}

// A box far narrower than it is tall, and one far wider, are what the position and size settings
// can cut out of a rotated panel. A face that only ever ran against something near square would not
// have met either.
func TestEveryFaceStaysInsideAnAwkwardBox(t *testing.T) {
	palette := theme.Default()
	r := Read(time.Date(2026, 9, 21, 10, 8, 0, 0, time.UTC), false)

	for _, box := range []ui.Rect{
		{X: 40, Y: 40, W: 300, H: 1200},
		{X: 40, Y: 40, W: 1600, H: 240},
		{X: 40, Y: 40, W: 200, H: 200},
	} {
		w, h := box.X+box.W+40, box.Y+box.H+40

		for _, name := range every() {
			blank := ui.NewImage(w, h, palette.Background)
			img := ui.NewImage(w, h, palette.Background)
			ui.Fill(img, palette.Background)

			Of(name).Draw(img, box, r, palette)

			if painted := ui.Changed(blank, img); !box.Holds(painted) {
				t.Errorf("%s in %+v painted %+v, outside it", name, box, painted)
			}
		}
	}
}
