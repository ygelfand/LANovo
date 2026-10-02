package scope

import "math"

// Bands is how many buckets a spectrum is reduced to. Enough to read as a shape, few enough that a
// narrow box still gives each one a column.
const Bands = 24

// Frame is a period of audio, already reduced to what drawing needs.
//
// Reduced once here rather than in each scope, so a host can hand the same frame to two of them and
// so the samples never reach the drawing code. A frame is cheap to compare, which is what lets a
// host redraw on change rather than on a timer: fifty periods a second from a quiet room all reduce
// to the same frame.
type Frame struct {
	// Peak and RMS are the loudest sample and the average power, both 0 to 1.
	Peak float64
	RMS  float64

	// Hold is the peak with the ballistics a meter needs, kept by Meter rather than computed per
	// frame. A VU meter without a hold is a bar that flickers.
	Hold float64

	// Columns is the waveform: the extent of the samples in each slice of time, low and high, so a
	// quiet passage is a thin line rather than a gap.
	Columns []Column

	// Bands is the spectrum, low frequency first, each 0 to 1.
	Bands []float64

	// Trail is the spectrum kept, oldest column first, filled by Keeper. Nil for a host that is
	// not keeping any.
	Trail [][]float64
}

// Column is the range the samples covered in one slice of time, each -1 to 1.
type Column struct{ Low, High float64 }

// Quiet reports whether there is nothing worth drawing. A host can skip a redraw on it, though the
// comparison that matters is between frames rather than against silence.
func (f Frame) Quiet() bool { return f.Peak == 0 }

// Read reduces one period of samples.
//
// columns is how many slices of time the waveform is cut into, which a host sets from the width of
// the box it is going to draw in: one column per pixel is the most that can be seen, and asking for
// more is work thrown away.
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

// cut is the waveform, as the extent of the samples in each slice.
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

// Meter is the ballistics a level needs to be readable: a level that followed the samples exactly
// would be unreadable, and one that only averaged would miss the transient that matters.
//
// Fast attack so a sound registers the instant it happens, slow decay so it can be read, and a peak
// that hangs and then falls.
type Meter struct {
	level float64
	hold  float64
	since int
}

// How the meter moves, per frame at the rate audio arrives.
const (
	attack = 0.5
	decay  = 0.08

	// hangFor is how many frames the peak stays put before it starts to fall. At fifty frames a
	// second this is about half a second, which is long enough to read a transient that is gone.
	hangFor = 25

	// fall is how fast the held peak drops once it lets go.
	fall = 0.04
)

// Next folds a frame into the meter and gives it back with the ballistics applied.
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

// Keeper is the history the spectrogram draws, one column of bands per frame.
//
// Held by the host the way Meter is, and for the same reason: a scope is one shared value with no
// room for anybody's state in it, and two hosts drawing the same sound each keep their own width
// of it.
type Keeper struct {
	past  [][]float64
	width int
}

// Next folds a frame into the trail and gives it back carrying it, oldest column first.
//
// width is how many columns to keep, which a host sets from the box it draws in. Changing it
// starts again rather than resampling: a box that has been resized is being redrawn anyway.
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
