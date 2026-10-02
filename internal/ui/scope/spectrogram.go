package scope

import (
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func init() { register(Spectrogram, sonogram{}) }

// hot is where the ramp leaves the ink for the accent. Below it a cell is the ink at a strength,
// above it the loudest cells separate from a field that is otherwise one color.
const hot = 0.7

type sonogram struct{}

// Draw is the spectrum kept: a column per frame, time left to right, low frequency at the bottom.
//
// The newest column is at the right and a short trail fills in from there, so a box that has not
// been fed for its whole width yet grows rather than stretching what little it has.
func (sonogram) Draw(s ui.Surface, in ui.Rect, f Frame, ink theme.Color, palette theme.Theme) {
	if in.W <= 0 || in.H <= 0 {
		return
	}

	ui.FillRect(s, in, palette.Surface)

	show := min(len(f.Trail), in.W)
	for i := range show {
		bands := f.Trail[len(f.Trail)-show+i]
		if len(bands) == 0 {
			continue
		}

		x := in.X + in.W - show + i
		for b, level := range bands {
			if level <= 0 {
				continue
			}

			// From the bottom up, the way the bars stand, so the two read the same way round.
			top := in.Y + in.H - (b+1)*in.H/len(bands)
			bottom := in.Y + in.H - b*in.H/len(bands)
			if bottom <= top {
				continue
			}

			ui.FillRect(s, ui.Rect{X: x, Y: top, W: 1, H: bottom - top}, shade(level, ink, palette))
		}
	}
}

// shade is a heat map in the theme's own colors, which is the only kind a theme here would put up
// with.
func shade(level float64, ink theme.Color, palette theme.Theme) theme.Color {
	if level <= hot {
		return palette.Surface.Blend(ink, level/hot)
	}
	return ink.Blend(palette.Accent, (level-hot)/(1-hot))
}
