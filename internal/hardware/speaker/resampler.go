package speaker

import "math"

// Resampler stretches a voice pipeline's 16 kHz mono to the 48 kHz stereo the codec takes.
//
// Unlike a microphone mix there is a right answer here, and it is the band limited one: repeating
// samples leaves images of the speech band in 8 to 16 kHz, which is heard as grit on consonants.
// The seam exists so the options can be compared on this speaker rather than argued about, and so
// there is somewhere cheap to fall back to if the filter ever costs more than it is worth.
type Resampler interface {
	// Run appends the interleaved stereo result of mono to out. It may keep state between calls, so
	// an utterance delivered in chunks comes out as one continuous signal.
	Run(mono []int16, out []int16) []int16

	// Reset drops that state, for when the next audio is unrelated to the last.
	Reset()

	// Clipped counts samples that came out past full scale.
	Clipped() uint64
}

// Resampling names one of them.
type Resampling string

const (
	ResampleSinc   Resampling = "Sinc"
	ResampleLinear Resampling = "Linear"
	ResampleHold   Resampling = "Hold"
)

// Resamplings is what this build can do, best first.
func Resamplings() []Resampling {
	return []Resampling{ResampleSinc, ResampleLinear, ResampleHold}
}

// NewResampler builds one, falling back to the filter for a name this build does not have.
func NewResampler(r Resampling) (Resampler, Resampling) {
	switch r {
	case ResampleLinear:
		return &linear{}, r
	case ResampleHold:
		return hold{}, r
	}
	return newSinc(), ResampleSinc
}

// linear draws a straight line between input samples: the images land where a held sample's would
// but come out attenuated, for two multiplies instead of a filter.
type linear struct{ prev int16 }

func (l *linear) Reset()        { l.prev = 0 }
func (*linear) Clipped() uint64 { return 0 }

func (l *linear) Run(mono []int16, out []int16) []int16 {
	for _, s := range mono {
		step := (float32(s) - float32(l.prev)) / VoiceUpsample
		for p := range VoiceUpsample {
			v := int16(float32(l.prev) + step*float32(p+1))
			out = append(out, v, v)
		}
		l.prev = s
	}
	return out
}

// hold repeats each input sample, which is the cheapest thing that fills the buffer.
type hold struct{}

func (hold) Reset()          {}
func (hold) Clipped() uint64 { return 0 }

func (hold) Run(mono []int16, out []int16) []int16 {
	for _, s := range mono {
		for range VoiceUpsample {
			out = append(out, s, s)
		}
	}
	return out
}

// A rising sweep is the easiest signal to hear the difference on: the images of a tone at f land at
// the input rate minus f, so as the sweep rises its ghost falls, and a second tone moving the wrong
// way is obvious in a way that grit on speech is not.
const (
	sweepMs   = 1500
	sweepFrom = 300.0
	sweepTo   = 7000.0
)

// VoiceSweep is that signal, at the rate a pipeline sends, so it goes through the resampler exactly
// as a reply does.
func VoiceSweep() []int16 {
	frames := VoiceRate * sweepMs / 1000
	out := make([]int16, frames)

	var phase float64
	for i := range out {
		hz := sweepFrom + (sweepTo-sweepFrom)*float64(i)/float64(frames)
		phase += 2 * math.Pi * hz / VoiceRate

		// Half scale, with the same edge ramp the chimes use so neither end clicks.
		env := math.Min(1, math.Min(float64(i), float64(frames-i))/float64(VoiceRate/50))
		out[i] = int16(0.5 * env * math.MaxInt16 * math.Sin(phase))
	}
	return out
}
