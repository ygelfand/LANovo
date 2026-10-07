package scope

import (
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func init() { register(VU, vu{}) }

// The meter's parts, as fractions of the box.
const (
	// markWide is the held peak's width against the box's long side.
	markWide = 0.012

	// trackShare is how much of the short side the bar fills, leaving the rest as air.
	trackShare = 0.55
)

type vu struct{}

// Draw is the level as a bar with the held peak marked beyond it.
//
// Along the longer side, so the same meter reads whether it is put in a strip across a screen or
// down the side of one.
func (vu) Draw(s ui.Surface, in ui.Rect, f Frame, ink theme.Color, palette theme.Theme) {
	if in.W <= 0 || in.H <= 0 {
		return
	}

	if in.W >= in.H {
		high := max(int(float64(in.H)*trackShare), 1)
		track := ui.Rect{X: in.X, Y: in.Y + (in.H-high)/2, W: in.W, H: high}

		ui.FillRect(s, track, palette.Surface)
		ui.FillRect(
			s,
			ui.Rect{X: track.X, Y: track.Y, W: int(f.RMS * float64(track.W)), H: track.H},
			ink,
		)

		wide := max(int(float64(in.W)*markWide), 1)
		at := track.X + min(int(f.Hold*float64(track.W)), track.W-wide)
		ui.FillRect(s, ui.Rect{X: at, Y: track.Y, W: wide, H: track.H}, palette.Accent)
		return
	}

	wide := max(int(float64(in.W)*trackShare), 1)
	track := ui.Rect{X: in.X + (in.W-wide)/2, Y: in.Y, W: wide, H: in.H}

	ui.FillRect(s, track, palette.Surface)

	// Up from the bottom, which is the way a level is read when it is vertical.
	high := int(f.RMS * float64(track.H))
	ui.FillRect(s, ui.Rect{X: track.X, Y: track.Y + track.H - high, W: track.W, H: high}, ink)

	thick := max(int(float64(in.H)*markWide), 1)
	at := track.Y + track.H - min(int(f.Hold*float64(track.H)), track.H-thick) - thick
	ui.FillRect(s, ui.Rect{X: track.X, Y: at, W: track.W, H: thick}, palette.Accent)
}
