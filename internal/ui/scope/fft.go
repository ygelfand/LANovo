package scope

import (
	"math"

	"github.com/ygelfand/libcountertop/pkg/audio/analysis"
)

// size is the transform's window, in samples. A power of two because the transform needs one, and
// 512 at 48kHz is about 11ms — short enough that a band follows speech rather than smearing it,
// long enough that the lowest bins mean something.
const size = 512

// spectrum reduces a period of samples to Bands buckets, low frequency first, each 0 to 1.
//
// Log frequency and log amplitude, because hearing is both. Linear bands would spend two thirds of
// the picture above 8kHz where there is nothing to see, and linear amplitude would leave everything
// but the loudest moment flat against the floor.
func spectrum(samples []int16) []float64 {
	out := make([]float64, Bands)
	if len(samples) == 0 {
		return out
	}

	re := make([]float64, size)
	im := make([]float64, size)

	// The last window of the period, so what is drawn is the most recent sound rather than an
	// average of the whole period. Zero padded when the period is short.
	from := max(len(samples)-size, 0)
	for i, s := range samples[from:] {
		// Hann, so a tone that does not fit a whole number of cycles in the window does not smear
		// itself across every band.
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

// band is the bin range one bucket covers, spaced logarithmically.
func band(b, half int) (lo, hi int) {
	at := func(i int) int {
		// From bin one rather than zero: bin zero is the average of the window, which is not a
		// frequency and is only ever a distraction on the left of the picture.
		v := math.Pow(float64(half-1), float64(i)/float64(Bands))
		return min(int(v), half-1)
	}

	lo, hi = at(b), at(b+1)
	if hi <= lo {
		hi = lo + 1
	}
	return lo, hi
}

// floorDB is where the picture bottoms out. Sixty below full scale is about as quiet as a room gets
// before the microphone's own noise is what is being drawn.
const floorDB = -60

// decibels puts a magnitude on a log scale between the floor and full scale.
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
