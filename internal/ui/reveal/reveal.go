package reveal

import (
	_ "embed"
	"image"
	"image/draw"
	"time"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

//go:embed reveal.glsl
var revealShader string

// Moment is the reveal's clock, the traced share of the arch, the move into the header and the finish.
type Moment struct {
	At                   time.Duration
	Trace, Header, Ready float64
}

type Reveal struct {
	logo *image.RGBA
	bg   theme.Color
	dark bool
}

func New(palette theme.Theme) *Reveal {
	dark := theme.Dark(palette.Background)
	src := ui.Logo()
	if dark {
		src = ui.Night()
	}
	logo := image.NewRGBA(src.Bounds())
	draw.Draw(logo, logo.Bounds(), src, src.Bounds().Min, draw.Src)
	return &Reveal{logo: logo, bg: palette.Background, dark: dark}
}

func centred(w, h int) (float32, float32, float32) {
	s := min(float32(w)*0.92/1000, float32(h)*0.62/666)
	return float32(w) / 2, float32(h) * 0.46, s
}

// Split is where the mark goes and where the list goes: beside it when wide, under it when tall.
func Split(w, h int) (logo, list ui.Rect) {
	if w >= h {
		at := int(float64(w) * LogoShare)
		return ui.Rect{W: at, H: h}, ui.Rect{X: at, W: w - at, H: h}
	}
	at := int(float64(h) * LogoShare)
	return ui.Rect{W: w, H: at}, ui.Rect{Y: at, W: w, H: h - at}
}

// LogoShare is how much of the long edge the mark takes once the list is up.
const LogoShare = 0.42

func header(w, h int) (float32, float32, float32) {
	logo, _ := Split(w, h)
	s := min(float32(logo.W)*0.86/1000, float32(logo.H)*0.86/666)
	return float32(logo.X) + float32(logo.W)/2, float32(logo.Y) + float32(logo.H)/2, s
}

func (r *Reveal) Shade(g visual.GL, fresh bool, w, h int, m Moment) error {
	if fresh {
		if err := g.Program(revealShader, visual.Light); err != nil {
			return err
		}
		if err := g.Texture(0, r.logo); err != nil {
			return err
		}
	}
	dark := float32(0)
	if r.dark {
		dark = 1
	}
	cx, cy, cs := centred(w, h)
	hx, hy, hs := header(w, h)
	vals := []float32{
		float32(m.At.Seconds()), float32(m.Trace), float32(m.Header), float32(m.Ready), dark,
		float32(r.bg.R) / 255, float32(r.bg.G) / 255, float32(r.bg.B) / 255,
		cx, cy, cs, hx, hy, hs,
	}
	return g.Values(vals, 0, 0, 0)
}
