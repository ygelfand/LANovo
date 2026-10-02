package visual

import (
	"math"
	"sync/atomic"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/analysis"
)

const (
	listening  = "listening"
	responding = "responding"
	idle       = "idle"
)

type frame struct {
	t, dt              float64
	state              string
	level, slow, voice float64
	onset              bool
	bands              [analysis.Bands]float64
	wave               [analysis.WavePoints]float64
}

const (
	loudSpeech = 0.65
	loudFloor  = 0.06
)

type framer struct {
	level, slow float64
	peak        float64
	onsets      uint64
	quiet       float64
}

func (fr *framer) next(x Input) frame {
	a := x.Voice()
	f := frame{t: x.Now.Seconds(), dt: math.Min(x.Dt.Seconds(), 0.1), voice: float64(a.Level)}
	fr.level = follow(fr.level, f.voice, 0.45, 0.12, f.dt)
	fr.slow = follow(fr.slow, f.voice, 0.06, 0.03, f.dt)
	fr.peak = math.Max(loudFloor, follow(fr.peak, fr.level, 0.9, 0.05, f.dt))
	g := gateFor(x.Replying)
	f.level = math.Min(1, loudSpeech*fr.level/fr.peak) * smoothstep(g.lo, g.hi, fr.level)
	f.slow = math.Min(1, loudSpeech*fr.slow/fr.peak) * smoothstep(g.lo, g.hi, fr.slow)
	f.onset = onset(&fr.onsets, a.Onsets)
	for b := range f.bands {
		f.bands[b] = float64(a.Bands[b])
	}
	for i := range f.wave {
		f.wave[i] = float64(a.Wave[i])
	}

	if f.voice > g.talk {
		fr.quiet = 0
	} else {
		fr.quiet += f.dt
	}
	switch {
	case fr.quiet >= 1.5:
		f.state = idle
	case x.Replying:
		f.state = responding
	default:
		f.state = listening
	}
	return f
}

func pow(a, b float64) float64 { return math.Pow(a, b) }

func noise1(x float64) float64 {
	i := math.Floor(x)
	t := x - i
	t = t * t * (3 - 2*t)
	a, b := noise(int(i), 17), noise(int(i)+1, 17)
	return a + (b-a)*t
}

func noise2(x, y float64) float64 {
	xi, yi := math.Floor(x), math.Floor(y)
	tx, ty := x-xi, y-yi
	tx, ty = tx*tx*(3-2*tx), ty*ty*(3-2*ty)
	ix, iy := int(xi), int(yi)
	a, b := noise(ix, iy*131+7), noise(ix+1, iy*131+7)
	c, d := noise(ix, (iy+1)*131+7), noise(ix+1, (iy+1)*131+7)
	top, bottom := a+(b-a)*tx, c+(d-c)*tx
	return top + (bottom-top)*ty
}

type rng struct{ s uint32 }

const SeedMost = 1024

var pinned atomic.Int32

func SetSeed(n int) { pinned.Store(int32(max(0, min(n, SeedMost)))) }

func seeded(base uint32) rng {
	if n := uint32(pinned.Load()); n > 0 {
		return rng{s: base*2654435761 ^ n*40503}
	}
	return rng{s: base ^ uint32(time.Now().UnixNano())}
}

func (r *rng) next() float64 {
	r.s = r.s*1664525 + 1013904223
	return float64(r.s>>8) / (1 << 24)
}

func (r *rng) gauss() float64 {
	u, v := math.Max(r.next(), 1e-9), r.next()
	return math.Sqrt(-2*math.Log(u)) * math.Cos(2*math.Pi*v)
}
