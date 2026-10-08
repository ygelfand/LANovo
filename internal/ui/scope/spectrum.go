package scope

import (
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func init() { register(Spectrum, bars{}) }

const gap = 0.25

type bars struct{}

func (bars) Draw(s ui.Surface, in ui.Rect, f Frame, ink theme.Color, _ theme.Theme) {
	if in.W <= 0 || in.H <= 0 || len(f.Bands) == 0 {
		return
	}

	show := min(len(f.Bands), in.W)
	step := float64(in.W) / float64(show)

	wide := max(int(step*(1-gap)), 1)

	for i := range show {
		var sum float64
		from := i * len(f.Bands) / show
		to := max((i+1)*len(f.Bands)/show, from+1)

		for _, v := range f.Bands[from:min(to, len(f.Bands))] {
			sum += v
		}
		level := sum / float64(to-from)

		high := max(int(level*float64(in.H)), 1)
		ui.FillRect(s, ui.Rect{
			X: in.X + int(float64(i)*step), Y: in.Y + in.H - high, W: wide, H: high,
		}, ink)
	}
}
