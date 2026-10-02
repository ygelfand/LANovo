package scope

import (
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func init() { register(Wave, wave{}) }

// thin is the least a column is drawn as, in pixels. A silent passage is a line through the middle
// rather than a gap: a waveform with holes in it reads as the drawing having failed.
const thin = 1

type wave struct{}

// Draw is the samples as their extent over time, mirrored about the middle of the box.
func (wave) Draw(s ui.Surface, in ui.Rect, f Frame, ink theme.Color, _ theme.Theme) {
	if in.W <= 0 || in.H <= 0 || len(f.Columns) == 0 {
		return
	}

	mid := in.Y + in.H/2
	half := float64(in.H) / 2

	for x := range in.W {
		c := f.Columns[x*len(f.Columns)/in.W]

		top := mid - int(c.High*half)
		bottom := mid - int(c.Low*half)
		if bottom-top < thin {
			top, bottom = mid-thin/2, mid-thin/2+thin
		}

		ui.FillRect(s, ui.Rect{X: in.X + x, Y: top, W: 1, H: bottom - top}, ink)
	}
}
