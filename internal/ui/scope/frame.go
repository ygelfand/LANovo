package scope

import "math"

const Bands = 24

type Frame struct {
	Peak float64
	RMS  float64

	Hold float64

	Columns []Column

	Bands []float64

	Trail [][]float64
}

type Column struct{ Low, High float64 }

func (f Frame) Quiet() bool { return f.Peak == 0 }

func Read(samples []int16, columns int) Frame {
	f := Frame{}
	if len(samples) == 0 {
		return f
	}

	var sum float64
	for _, s := range samples {
		v := float64(s) / math.MaxInt16
		sum += v * v

		if a := math.Abs(v); a > f.Peak {
			f.Peak = a
		}
	}
	f.RMS = math.Sqrt(sum / float64(len(samples)))
	f.Hold = f.Peak

	if columns > 0 {
		f.Columns = cut(samples, columns)
	}
	f.Bands = spectrum(samples)
	return f
}

func cut(samples []int16, columns int) []Column {
	if columns > len(samples) {
		columns = len(samples)
	}

	out := make([]Column, columns)
	for i := range out {
		from := i * len(samples) / columns
		to := (i + 1) * len(samples) / columns
		if to <= from {
			to = from + 1
		}

		low, high := math.Inf(1), math.Inf(-1)
		for _, s := range samples[from:min(to, len(samples))] {
			v := float64(s) / math.MaxInt16
			low, high = math.Min(low, v), math.Max(high, v)
		}
		out[i] = Column{Low: low, High: high}
	}
	return out
}

type Meter struct {
	level float64
	hold  float64
	since int
}

const (
	attack = 0.5
	decay  = 0.08

	hangFor = 25

	fall = 0.04
)

func (m *Meter) Next(f Frame) Frame {
	rate := decay
	if f.RMS > m.level {
		rate = attack
	}
	m.level += (f.RMS - m.level) * rate

	switch {
	case f.Peak >= m.hold:
		m.hold, m.since = f.Peak, 0
	case m.since < hangFor:
		m.since++
	default:
		m.hold = math.Max(m.level, m.hold-fall)
	}

	f.RMS, f.Hold = m.level, m.hold
	return f
}

type Keeper struct {
	past  [][]float64
	width int
}

func (k *Keeper) Next(f Frame, width int) Frame {
	if width <= 0 {
		return f
	}
	if width != k.width {
		k.past, k.width = make([][]float64, 0, width), width
	}

	if len(k.past) < width {
		k.past = append(k.past, f.Bands)
	} else {
		copy(k.past, k.past[1:])
		k.past[width-1] = f.Bands
	}

	f.Trail = k.past
	return f
}
