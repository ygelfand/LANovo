package scope

import (
	"math"

	"github.com/ygelfand/libcountertop/pkg/audio/analysis"
)

const size = 512

func spectrum(samples []int16) []float64 {
	out := make([]float64, Bands)
	if len(samples) == 0 {
		return out
	}

	re := make([]float64, size)
	im := make([]float64, size)

	from := max(len(samples)-size, 0)
	for i, s := range samples[from:] {
		w := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(size-1))
		re[i] = float64(s) / math.MaxInt16 * w
	}

	analysis.Transform(re, im)

	// Only the first half is meaningful; above it is the mirror of below.
	half := size / 2
	for b := range Bands {
		lo, hi := band(b, half)

		var loudest float64
		for i := lo; i < hi; i++ {
			if m := math.Hypot(re[i], im[i]); m > loudest {
				loudest = m
			}
		}
		out[b] = decibels(loudest)
	}
	return out
}

func band(b, half int) (lo, hi int) {
	at := func(i int) int {
		// Bin zero is the window's mean.
		v := math.Pow(float64(half-1), float64(i)/float64(Bands))
		return min(int(v), half-1)
	}

	lo, hi = at(b), at(b+1)
	if hi <= lo {
		hi = lo + 1
	}
	return lo, hi
}

const floorDB = -60

func decibels(v float64) float64 {
	if v <= 0 {
		return 0
	}

	db := 20 * math.Log10(v)
	if db <= floorDB {
		return 0
	}
	return min(1, (db-floorDB)/-floorDB)
}
