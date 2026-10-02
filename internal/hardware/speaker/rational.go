package speaker

import "math"

// Resampling between two rates that are not multiples of each other.
//
// The card runs at one rate and will not change: it is opened once and shared, so whatever wants to
// play has to arrive at 48 kHz. Bluetooth audio usually does not — a phone picks 44.1 for SBC — and
// 44100 played as 48000 is eight percent fast, which is most of a semitone.
//
// 44100 to 48000 is 147 to 160, so there is no whole number of output samples per input one. The
// voice path upsamples by exactly three and can hold the ratio in a constant; this cannot.

// ratioTaps is how many input samples each output one is made from.
//
// Sixteen puts the images far enough down to be inaudible against what SBC already contributes, and
// costs sixteen multiplies per sample per channel — about 1.5 million a second for stereo at 48 kHz,
// which this device does not notice.
const ratioTaps = 16

// Rational resamples interleaved audio by a ratio of whole numbers.
//
// It carries the tail of what it was last given, because an output sample near the start of a buffer
// is made partly from the end of the one before. One per stream: handing it a different stream's
// audio joins the two together through the filter.
type Rational struct {
	up   int // output samples per this many input ones
	down int

	channels int

	// filter is the low pass, laid out by phase: the taps for phase p are filter[p], filter[p+up],
	// and so on. Only one phase is used per output sample, which is what makes this affordable.
	filter []float32

	// at is where we are in the upsampled stream, in units of one input sample over up.
	at int

	// tail is the end of the last buffer, kept because the next one continues from it.
	tail []int16

	skew, owed float64

	// clipped counts samples that came out past full scale. An interpolating filter overshoots a
	// transient by a few percent, so audio arriving near full scale clips here — and since volume
	// is applied further along, that distortion survives being turned down.
	clipped uint64
}

// NewRational resamples from one rate to another, for interleaved audio of some channels.
//
// Rates that are already equal give a Rational that hands its input straight back, so a caller can
// make one unconditionally rather than deciding.
func NewRational(from, to, channels int) *Rational {
	if channels < 1 {
		channels = 1
	}

	g := gcd(from, to)
	r := &Rational{up: to / g, down: from / g, channels: channels}

	if from != to {
		r.filter = lowpass(r.up, ratioTaps)
	}
	r.Reset()
	return r
}

const skewGrid = 160

func NewSkewable(from, to, channels int) *Rational {
	if from != to {
		return NewRational(from, to, channels)
	}
	r := &Rational{up: skewGrid, down: skewGrid, channels: max(channels, 1), filter: lowpass(skewGrid, ratioTaps)}
	r.Reset()
	return r
}

// Headroom scales the filter, for a path whose input arrives near full scale.
//
// Interpolation overshoots a transient by a few percent, so audio that is already loud clips in the
// filter rather than at the speaker. Taking a little off costs less than a decibel and the volume
// control downstream makes even that moot.
func (r *Rational) Headroom(g float32) {
	for i := range r.filter {
		r.filter[i] *= g
	}
}

// Ratio is what it converts, as the whole numbers it reduced to.
func (r *Rational) Ratio() (up, down int) { return r.up, r.down }

// Run resamples one buffer. The result is fresh each call and is not held onto.
func (r *Rational) Run(in []int16) []int16 {
	if r.filter == nil || len(in) == 0 {
		return in
	}

	ch := r.channels
	buf := append(r.tail, in...)
	frames := len(buf) / ch

	// Roughly what is coming, so the slice does not grow through the loop.
	out := make([]int16, 0, (len(in)*r.up/r.down)+ch)

	for {
		// The newest input frame this output sample needs, and which phase of the filter applies.
		idx := r.at / r.up
		if idx >= frames {
			break
		}
		if idx < ratioTaps-1 {
			// Not enough history yet, which only happens at the very start of a stream.
			r.at += r.down
			continue
		}

		phase := r.at % r.up
		for c := range ch {
			var sum float32
			for k := range ratioTaps {
				sum += r.filter[phase+k*r.up] * float32(buf[(idx-k)*ch+c])
			}
			if sum > 32767 || sum < -32768 {
				r.clipped++
			}
			out = append(out, clip(sum))
		}
		r.at += r.step()
	}

	// Keep only what the next call still needs: the taps behind wherever we got to.
	idx := min(r.at/r.up, frames)
	drop := max(idx-(ratioTaps-1), 0)

	r.at -= drop * r.up
	r.tail = append(r.tail[:0], buf[drop*ch:]...)

	return out
}

// Reset forgets the tail, for a stream that has stopped. The next one starts from silence rather
// than from the end of the last.
//
// Silence is put there rather than left empty: an output sample is made from the taps behind it, so
// a stream starting with nothing behind it would have its first ratioTaps-1 frames swallowed while
// the history filled. Beginning against zeroes is what a stream starting from quiet means, and it
// makes the first input sample produce output.
func (r *Rational) Reset() {
	r.at = 0
	r.skew, r.owed = 0, 0

	r.tail = append(r.tail[:0], make([]int16, (ratioTaps-1)*r.channels)...)
	r.at = (ratioTaps - 1) * r.up
}

// Clipped counts samples that came out past full scale.
func (r *Rational) Clipped() uint64 { return r.clipped }

func (r *Rational) Skew(ppm float64) { r.skew = ppm * 1e-6 * float64(r.down) }

func (r *Rational) step() int {
	r.owed += r.skew
	extra := int(r.owed)
	r.owed -= float64(extra)
	return r.down + extra
}

// lowpass is a windowed sinc at the lower of the two Nyquists, laid out for polyphase use.
//
// Cutoff is half the input rate rather than half the output one whenever the input is the narrower,
// which for anything upsampling it is. Anything wider passes images of the input band.
func lowpass(phases, taps int) []float32 {
	n := phases * taps
	h := make([]float32, n)

	center := float64(n-1) / 2
	for i := range h {
		x := (float64(i) - center) / float64(phases)

		s := 1.0
		if x != 0 {
			s = math.Sin(math.Pi*x) / (math.Pi * x)
		}

		// Hamming, as the voice filter uses: a little transition width for a stopband below what
		// the codec contributes anyway.
		w := 0.54 - 0.46*math.Cos(2*math.Pi*float64(i)/float64(n-1))
		h[i] = float32(s * w)
	}

	// Each phase has to sum to one, or the result is quieter or louder than what came in. The
	// window takes a little off, and which it takes differs per phase.
	for p := range phases {
		var sum float32
		for k := range taps {
			sum += h[p+k*phases]
		}
		if sum == 0 {
			continue
		}
		for k := range taps {
			h[p+k*phases] /= sum
		}
	}
	return h
}

// clip saturates rather than wrapping. Interpolation overshoots a transient by a few percent, and a
// wrapped sample is full scale of the opposite sign, which is a crack rather than a loud moment.
func clip(v float32) int16 {
	switch {
	case v > 32767:
		return 32767
	case v < -32768:
		return -32768
	}
	return int16(v)
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}
