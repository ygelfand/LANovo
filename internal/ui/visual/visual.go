package visual

import (
	"image"
	"slices"
	"strings"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/analysis"
	"github.com/ygelfand/LANovo/internal/lib/say"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

type Kind string

const (
	LensFlares   Kind = "lens-flares"
	ClassicVU    Kind = "classic-vu"
	Aurora       Kind = "aurora"
	Mercury      Kind = "mercury"
	Ribbons      Kind = "ribbons"
	Galaxy       Kind = "galaxy"
	Halo         Kind = "halo"
	Pulsar       Kind = "pulsar"
	Phosphor     Kind = "phosphor"
	Tesla        Kind = "tesla"
	Orb          Kind = "orb"
	DigitalVU    Kind = "digital-vu"
	PaintSplash  Kind = "paint-splash"
	Fireworks    Kind = "fireworks"
	Synthwave    Kind = "synthwave"
	HAL9000      Kind = "hal-9000"
	Mother       Kind = "mother"
	LightCycles  Kind = "light-cycles"
	LCARS        Kind = "lcars"
	PipBoy3      Kind = "pipboy-3"
	PipBoyNV     Kind = "pipboy-nv"
	PipBoy4      Kind = "pipboy-4"
	PipBoy76     Kind = "pipboy-76"
	KoiPond      Kind = "koi-pond"
	RainOnGlass  Kind = "rain-on-glass"
	Cymatics     Kind = "cymatics"
	Fire         Kind = "fire"
	NeonSign     Kind = "neon-sign"
	LavaLamp     Kind = "lava-lamp"
	InkInWater   Kind = "ink-in-water"
	Thunderstorm Kind = "thunderstorm"
	Fireflies    Kind = "fireflies"
	Matrix       Kind = "matrix"
)

const Default = ClassicVU

var order = []Kind{
	LensFlares, ClassicVU, Aurora, Mercury, Ribbons, Galaxy, Halo, Pulsar, Phosphor, Tesla, Orb,
	DigitalVU, PaintSplash, Fireworks, Synthwave, HAL9000, Mother, LightCycles, LCARS, PipBoy3, PipBoyNV, PipBoy4, PipBoy76, KoiPond, RainOnGlass, Cymatics, Fire, NeonSign, LavaLamp, InkInWater, Thunderstorm, Fireflies, Matrix,
}

type Traits struct {
	Themed bool
	Light  bool
}

var traits = map[Kind]Traits{
	LensFlares:   {},
	ClassicVU:    {},
	Aurora:       {},
	Mercury:      {},
	Ribbons:      {},
	Galaxy:       {},
	Halo:         {},
	Pulsar:       {},
	Phosphor:     {},
	Tesla:        {},
	Orb:          {},
	DigitalVU:    {},
	PaintSplash:  {},
	Fireworks:    {},
	Synthwave:    {},
	HAL9000:      {},
	Mother:       {},
	LightCycles:  {},
	LCARS:        {},
	PipBoy3:      {},
	PipBoyNV:     {},
	PipBoy4:      {},
	PipBoy76:     {},
	KoiPond:      {},
	RainOnGlass:  {},
	Cymatics:     {},
	Fire:         {},
	NeonSign:     {},
	LavaLamp:     {},
	InkInWater:   {},
	Thunderstorm: {},
	Fireflies:    {},
	Matrix:       {},
}

func (k Kind) Traits() Traits { return traits[k] }

func (t Traits) Under(palette theme.Theme) theme.Theme {
	if t.Themed {
		return palette
	}
	return palette.On(!t.Light)
}

type Input struct {
	Mic, Speaker analysis.Analysis

	MicLeft, MicRight         analysis.Analysis
	SpeakerLeft, SpeakerRight analysis.Analysis

	Replying bool

	Now, Dt time.Duration

	Label string
}

type gate struct{ lo, hi, talk float64 }

var (
	micGate     = gate{lo: 0.012, hi: 0.035, talk: 0.03}
	speakerGate = gate{lo: 0.0002, hi: 0.001, talk: 0.0002}
)

func gateFor(replying bool) gate {
	if replying {
		return speakerGate
	}
	return micGate
}

func (x Input) Voice() analysis.Analysis {
	if x.Mic == x.Speaker {
		return single(x.Mic, gateFor(x.Replying))
	}
	return merged(x.Mic, x.Speaker)
}

func single(a analysis.Analysis, g gate) analysis.Analysis {
	out := analysis.Analysis{Level: a.Level, Peak: a.Peak, Onsets: a.Onsets}
	out.Wave, out.Bands = scaled(a, g)
	return out
}

func merged(a, b analysis.Analysis) analysis.Analysis {
	out := analysis.Analysis{Level: max(a.Level, b.Level), Peak: max(a.Peak, b.Peak), Onsets: a.Onsets + b.Onsets}
	aw, ab := scaled(a, micGate)
	bw, bb := scaled(b, speakerGate)
	for i := range out.Bands {
		out.Bands[i] = max(ab[i], bb[i])
	}
	for i := range out.Wave {
		out.Wave[i] = min(max(aw[i]+bw[i], -1), 1)
	}
	return out
}

func scaled(a analysis.Analysis, g gate) (wave [analysis.WavePoints]float32, bands [analysis.Bands]float32) {
	gate := float32(smoothstep(g.lo, g.hi, float64(a.Level)))
	if gate <= 0 {
		return wave, bands
	}
	var pw, pb float32
	for _, v := range a.Wave {
		pw = max(pw, v, -v)
	}
	for _, v := range a.Bands {
		pb = max(pb, v)
	}
	for i, v := range a.Wave {
		if pw > 0 {
			wave[i] = v / pw * gate
		}
	}
	for i, v := range a.Bands {
		if pb > 0 {
			bands[i] = v / pb * gate
		}
	}
	return wave, bands
}

type Visual interface {
	Shade(g GL, fresh bool, in ui.Rect, x Input) error
}

type Passes uint32

const (
	Light Passes = 1 << iota
	Pre
	Feed
	FeedHalf
	Splat
	SplatHalf
	Lines
	FeedFloat
)

type GL interface {
	Texture(unit int, img *image.RGBA) error
	Program(src string, passes Passes) error
	Values(u []float32, amount, radius float32, passes int) error
	Points(p []Point) error
	Lines(s []Segment) error
	Quads(q []Quad) error
}

type Quad struct {
	X, Y       [4]float32
	R, G, B, A float32
}

type Segment struct {
	X0, Y0, X1, Y1, Width float32
	R, G, B, A            float32
}

type Point struct {
	X, Y, Size float32
	R, G, B, A float32
	Feed       bool
	Round      bool
	Light      bool
	Splat      bool
	Disc       bool
	Over       bool
}

var registered = map[Kind]func() Visual{}

func register(k Kind, make func() Visual) { registered[k] = make }

func New(k Kind) Visual {
	if make, ok := registered[k]; ok {
		return make()
	}
	return registered[Default]()
}

func Kinds() []Kind { return order }

func Built() []Kind {
	var out []Kind
	for _, k := range order {
		if _, ok := registered[k]; ok {
			out = append(out, k)
		}
	}
	slices.SortFunc(out, func(a, b Kind) int { return strings.Compare(strings.ToLower(a.Label()), strings.ToLower(b.Label())) })
	return out
}

func (k Kind) Label() string {
	if _, ok := traits[k]; !ok {
		k = Default
	}
	return say.T("visual." + string(k))
}

type Layout int

const (
	Full Layout = iota
	Compact
)

func LayoutOf(in, screen ui.Rect) Layout {
	if in.W*in.H*2 < screen.W*screen.H {
		return Compact
	}
	return Full
}

func Portrait(in ui.Rect) bool { return in.H > in.W }
