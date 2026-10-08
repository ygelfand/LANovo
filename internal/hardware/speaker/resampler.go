package speaker

import "math"

type Resampler interface {
	Run(mono []int16, out []int16) []int16

	Reset()

	Clipped() uint64
}

type Resampling string

const (
	ResampleSinc   Resampling = "Sinc"
	ResampleLinear Resampling = "Linear"
	ResampleHold   Resampling = "Hold"
)

func Resamplings() []Resampling {
	return []Resampling{ResampleSinc, ResampleLinear, ResampleHold}
}

func NewResampler(r Resampling) (Resampler, Resampling) {
	switch r {
	case ResampleLinear:
		return &linear{}, r
	case ResampleHold:
		return hold{}, r
	}
	return newSinc(), ResampleSinc
}

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

const (
	sweepMs   = 1500
	sweepFrom = 300.0
	sweepTo   = 7000.0
)

func VoiceSweep() []int16 {
	frames := VoiceRate * sweepMs / 1000
	out := make([]int16, frames)

	var phase float64
	for i := range out {
		hz := sweepFrom + (sweepTo-sweepFrom)*float64(i)/float64(frames)
		phase += 2 * math.Pi * hz / VoiceRate

		env := math.Min(1, math.Min(float64(i), float64(frames-i))/float64(VoiceRate/50))
		out[i] = int16(0.5 * env * math.MaxInt16 * math.Sin(phase))
	}
	return out
}
