package visuals

import (
	"math"

	"github.com/ygelfand/libcountertop/pkg/audio/analysis"
)

const LiftMax = 40

func liftGain(db int) float32 { return float32(math.Pow(10, float64(db)/20)) }

type stereo struct {
	left, right, mix *analysis.Analyzer
	l, r, m          []int16
}

func newStereo(rate int, gain float32) *stereo {
	s := &stereo{left: analysis.New(rate), right: analysis.New(rate), mix: analysis.New(rate)}
	s.setGain(gain)
	return s
}

func (s *stereo) setGain(g float32) {
	for _, a := range []*analysis.Analyzer{s.left, s.right, s.mix} {
		a.SetGain(g)
	}
}

func (s *stereo) write(interleaved []int16) {
	n := len(interleaved) / 2
	s.l, s.r, s.m = s.l[:0], s.r[:0], s.m[:0]
	for i := range n {
		a, b := interleaved[2*i], interleaved[2*i+1]
		s.l = append(s.l, a)
		s.r = append(s.r, b)
		s.m = append(s.m, int16((int32(a)+int32(b))/2))
	}
	s.left.Write(s.l)
	s.right.Write(s.r)
	s.mix.Write(s.m)
}

func (s *stereo) reset() {
	s.left.Reset()
	s.right.Reset()
	s.mix.Reset()
}
