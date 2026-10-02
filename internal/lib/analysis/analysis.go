package analysis

import (
	"math"
	"sync"
	"sync/atomic"
)

const (
	Bands      = 32
	WavePoints = 256

	lowHz  = 60.0
	highHz = 8000.0

	floorDB = -60.0

	attack = 0.5
	decay  = 0.08

	onsetRise  = 1.5
	onsetRearm = 1.1
	onsetFloor = 0.02
	onsetGapMs = 120

	analysisRate = 16000
)

type Analysis struct {
	Level  float32
	Peak   float32
	Bands  [Bands]float32
	Wave   [WavePoints]float32
	Onsets uint64
}

type Analyzer struct {
	rate, size, hop int

	factor, summed int
	sum            int32

	hist []float32
	pos  int
	fill int

	re, im []float64
	window []float64
	edges  [Bands + 1]int

	gain atomic.Uint32

	level, fast, slow float64
	since, gap        int
	armed             bool
	onsets            uint64

	mu  sync.Mutex
	out Analysis
}

func New(rate int) *Analyzer {
	factor := 1
	if rate > analysisRate && rate%analysisRate == 0 {
		factor, rate = rate/analysisRate, analysisRate
	}
	size := 1
	for size < rate*32/1000 {
		size <<= 1
	}
	a := &Analyzer{
		rate: rate, size: size, hop: max(rate/100, 1), factor: factor, armed: true,
		hist: make([]float32, size),
		re:   make([]float64, size), im: make([]float64, size),
		window: make([]float64, size),
		gap:    rate * onsetGapMs / 1000,
	}
	for i := range a.window {
		a.window[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(size-1))
	}

	half := size / 2
	top := math.Min(highHz, float64(rate)/2)
	for b := range a.edges {
		hz := lowHz * math.Pow(top/lowHz, float64(b)/Bands)
		a.edges[b] = min(max(int(hz*float64(size)/float64(rate)), 1), half-1)
	}
	for b := 1; b < len(a.edges); b++ {
		a.edges[b] = max(a.edges[b], a.edges[b-1]+1)
	}
	return a
}

func (a *Analyzer) Rate() int { return a.rate }

func (a *Analyzer) SetGain(g float32) { a.gain.Store(math.Float32bits(g)) }

func (a *Analyzer) Write(mono []int16) {
	gain := float32(1)
	if bits := a.gain.Load(); bits != 0 {
		gain = math.Float32frombits(bits)
	}
	for _, s := range mono {
		if a.factor > 1 {
			a.sum += int32(s)
			if a.summed++; a.summed < a.factor {
				continue
			}
			s = int16(a.sum / int32(a.factor))
			a.sum, a.summed = 0, 0
		}
		a.hist[a.pos] = float32(s) / math.MaxInt16 * gain
		a.pos = (a.pos + 1) % a.size
		a.since++
		if a.fill++; a.fill >= a.hop {
			a.fill = 0
			a.step()
		}
	}
}

func (a *Analyzer) Latest() Analysis {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.out
}

func (a *Analyzer) Reset() {
	clear(a.hist)
	a.pos, a.fill = 0, 0
	a.sum, a.summed = 0, 0
	a.level, a.fast, a.slow = 0, 0, 0
	a.armed = true
	a.mu.Lock()
	a.out = Analysis{Onsets: a.onsets}
	a.mu.Unlock()
}

func (a *Analyzer) at(i int) float32 { return a.hist[(a.pos+i)%a.size] }

func (a *Analyzer) step() {
	var out Analysis

	var sum, peak float64
	for i := a.size - a.hop; i < a.size; i++ {
		v := float64(a.at(i))
		sum += v * v
		peak = math.Max(peak, math.Abs(v))
	}
	rms := math.Sqrt(sum / float64(a.hop))

	rate := decay
	if rms > a.level {
		rate = attack
	}
	a.level += (rms - a.level) * rate
	out.Level, out.Peak = float32(math.Min(1, a.level*math.Sqrt2)), float32(peak)

	for i := range a.size {
		a.re[i], a.im[i] = float64(a.at(i))*a.window[i], 0
	}
	Transform(a.re, a.im)
	scale := 4 / float64(a.size)
	for b := range Bands {
		var loudest float64
		for i := a.edges[b]; i < a.edges[b+1]; i++ {
			loudest = math.Max(loudest, math.Hypot(a.re[i], a.im[i]))
		}
		out.Bands[b] = decibels(loudest * scale)
	}

	for i := range WavePoints {
		out.Wave[i] = max(-1, min(1, a.at(i*a.size/WavePoints)))
	}

	a.fast += (rms - a.fast) * 0.5
	a.slow += (rms - a.slow) * 0.05
	switch {
	case a.armed && a.fast > a.slow*onsetRise && a.fast > onsetFloor && a.since >= a.gap:
		a.onsets++
		a.since, a.armed = 0, false
	case !a.armed && a.fast < a.slow*onsetRearm:
		a.armed = true
	}
	out.Onsets = a.onsets

	a.mu.Lock()
	a.out = out
	a.mu.Unlock()
}

func decibels(v float64) float32 {
	if v <= 0 {
		return 0
	}
	db := 20 * math.Log10(v)
	if db <= floorDB {
		return 0
	}
	return float32(math.Min(1, (db-floorDB)/-floorDB))
}

func Mono(interleaved []int16, channels int, out []int16) []int16 {
	out = out[:0]
	if channels <= 1 {
		return append(out, interleaved...)
	}
	for i := 0; i+channels <= len(interleaved); i += channels {
		var sum int32
		for c := range channels {
			sum += int32(interleaved[i+c])
		}
		out = append(out, int16(sum/int32(channels)))
	}
	return out
}
