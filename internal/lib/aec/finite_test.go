package aec

import (
	"math"
	"math/cmplx"
	"testing"
)

func TestStateStaysFiniteUnderAdversarialInput(t *testing.T) {
	const (
		frames = 2000
		hop    = 320
	)

	patterns := []struct {
		name     string
		mic, ref func(i int) int16
	}{
		{"silence", func(int) int16 { return 0 }, func(int) int16 { return 0 }},
		{"loud mic, silent reference", func(i int) int16 { return int16(30000 * math.Sin(float64(i)/8)) }, func(int) int16 { return 0 }},
		{"silent mic, loud reference", func(int) int16 { return 0 }, func(i int) int16 { return int16(30000 * math.Sin(float64(i)/8)) }},
		{"full scale square", sq, sq},
		{"dc at full scale", func(int) int16 { return 32767 }, func(int) int16 { return 32767 }},
		{"alternating extremes", alt, alt},
		{"one lsb", func(int) int16 { return 1 }, func(int) int16 { return 1 }},
		{"loud reference, one lsb mic", func(int) int16 { return 1 }, sq},
		{"reference uncorrelated with mic", func(i int) int16 { return int16(20000 * math.Sin(float64(i)/3)) }, func(i int) int16 { return int16(20000 * math.Sin(float64(i)/97)) }},
	}

	for _, p := range patterns {
		c, err := NewMDF(64, 1024, 16000)
		if err != nil {
			t.Fatal(err)
		}

		mic := make([]int16, hop)
		ref := make([]int16, hop)
		n := 0
		for range frames {
			for i := range mic {
				mic[i], ref[i] = p.mic(n+i), p.ref(n+i)
			}
			n += hop
			if _, err := c.Process(mic, ref); err != nil {
				t.Fatalf("%s: %v", p.name, err)
			}
		}

		bad := 0
		for _, part := range c.w {
			for _, v := range part {
				if cmplx.IsNaN(complex128(v)) || cmplx.IsInf(complex128(v)) {
					bad++
				}
			}
		}
		if bad > 0 {
			t.Errorf("%s: %d filter weights are non-finite", p.name, bad)
		}
		for what, v := range map[string]float64{"sumD": c.sumD, "sumE": c.sumE, "ERLE": c.ERLE()} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Errorf("%s: %s is %v", p.name, what, v)
			}
		}

		var loudest int16
		for range 200 {
			for i := range mic {
				mic[i] = int16(8000 * math.Sin(2*math.Pi*440*float64(n+i)/16000))
				ref[i] = 0
			}
			n += hop
			out, err := c.Process(mic, ref)
			if err != nil {
				t.Fatalf("%s: %v", p.name, err)
			}
			for _, v := range out {
				loudest = max(loudest, v)
			}
		}
		if loudest == 0 {
			t.Errorf("%s: a tone afterwards comes out as pure zeros", p.name)
		}
	}
}

func sq(i int) int16 {
	if i/16%2 == 0 {
		return 32767
	}
	return -32768
}

func alt(i int) int16 {
	if i%2 == 0 {
		return 32767
	}
	return -32768
}
