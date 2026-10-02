package visuals

import (
	"math"
	"testing"
)

func leftOnly(amp float64, frames int) []int16 {
	out := make([]int16, frames*2)
	for i := range frames {
		out[2*i] = int16(amp * math.Sin(2*math.Pi*500*float64(i)/48000))
	}
	return out
}

func TestEachChannelReadsItsOwnMicrophone(t *testing.T) {
	s := newStereo(48000, 1)
	for range 20 {
		s.write(leftOnly(8000, 960))
	}
	l, r, m := s.left.Latest().Level, s.right.Latest().Level, s.mix.Latest().Level
	if r > l/100 {
		t.Errorf("the silent right channel reads %.3f against the left's %.3f", r, l)
	}
	if math.Abs(float64(m)-float64(l)/2) > float64(l)*0.05 {
		t.Errorf("the mix reads %.3f, want half the left's %.3f", m, l)
	}
}

func TestTheCalibrationScalesWithoutClipping(t *testing.T) {
	plain, lifted := newStereo(48000, 1), newStereo(48000, 4)
	for range 20 {
		plain.write(leftOnly(4000, 960))
		lifted.write(leftOnly(4000, 960))
	}
	p, q := plain.left.Latest().Level, lifted.left.Latest().Level
	if math.Abs(float64(q)-4*float64(p)) > float64(p)*0.1 {
		t.Errorf("a gain of 4 read %.3f, want 4× %.3f", q, p)
	}
	for i, w := range lifted.left.Latest().Wave {
		if w > 1 || w < -1 {
			t.Fatalf("wave point %d is %.2f, outside the drawable range", i, w)
		}
	}
}
