package aec

import (
	"math"
	"math/rand/v2"
)

const frame = 320

func echoed(ref []int16, h []float64) []int16 {
	out := make([]int16, len(ref))
	for i := range ref {
		var v float64
		for k, c := range h {
			if i-k >= 0 {
				v += c * float64(ref[i-k])
			}
		}
		out[i] = int16(v)
	}
	return out
}

func noiseAt(n int, amp float64, seed uint64) []int16 {
	out := make([]int16, n)
	r := rand.New(rand.NewPCG(seed, 7))
	for i := range out {
		out[i] = int16((r.Float64()*2 - 1) * amp)
	}
	return out
}

func erle(mic, out []int16) float64 {
	var d, e float64
	for i := range mic {
		d += float64(mic[i]) * float64(mic[i])
		e += float64(out[i]) * float64(out[i])
	}
	if e == 0 {
		return math.Inf(1)
	}
	return 10 * math.Log10(d/e)
}
