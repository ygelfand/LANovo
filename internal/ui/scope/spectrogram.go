package scope

import (
	"github.com/ygelfand/libcountertop/pkg/display/theme"
	"github.com/ygelfand/libcountertop/pkg/display/ui"
)

func init() { register(Spectrogram, sonogram{}) }

const hot = 0.7

type sonogram struct{}

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

			top := in.Y + in.H - (b+1)*in.H/len(bands)
			bottom := in.Y + in.H - b*in.H/len(bands)
			if bottom <= top {
				continue
			}

			ui.FillRect(s, ui.Rect{X: x, Y: top, W: 1, H: bottom - top}, shade(level, ink, palette))
		}
	}
}

func shade(level float64, ink theme.Color, palette theme.Theme) theme.Color {
	if level <= hot {
		return palette.Surface.Blend(ink, level/hot)
	}
	return ink.Blend(palette.Accent, (level-hot)/(1-hot))
}
