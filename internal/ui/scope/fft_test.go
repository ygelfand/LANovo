package scope

import (
	"math"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/audio/analysis"
)

func tone(hz, rate float64, n int, amp float64) []int16 {
	out := make([]int16, n)
	for i := range out {
		out[i] = int16(amp * math.MaxInt16 * math.Sin(2*math.Pi*hz*float64(i)/rate))
	}
	return out
}

func loudest(bands []float64) int {
	at := 0
	for i, v := range bands {
		if v > bands[at] {
			at = i
		}
	}
	return at
}

func TestASineLandsInItsOwnBin(t *testing.T) {
	const rate = 48000

	re := make([]float64, size)
	im := make([]float64, size)

	const cycles = 8
	for i := range re {
		re[i] = math.Sin(2 * math.Pi * cycles * float64(i) / float64(size))
	}

	analysis.Transform(re, im)

	at, most := 0, 0.0
	for i := range size / 2 {
		if m := math.Hypot(re[i], im[i]); m > most {
			at, most = i, m
		}
	}
	if at != cycles {
		t.Errorf("a tone of %d cycles peaked in bin %d, want %d", cycles, at, cycles)
	}

	for i := range size / 2 {
		if i == cycles {
			continue
		}
		if m := math.Hypot(re[i], im[i]); m > most/10 {
			t.Errorf("bin %d holds %.1f%% of the peak, want a single tone in a single bin",
				i, 100*m/most)
		}
	}
	_ = rate
}

func TestSilenceHasNoSpectrum(t *testing.T) {
	for i, v := range spectrum(make([]int16, size)) {
		if v != 0 {
			t.Errorf("band %d reads %v in silence, want 0", i, v)
		}
	}
}

func TestLowAndHighSitAtOppositeEnds(t *testing.T) {
	low := loudest(spectrum(tone(200, 48000, size, 0.8)))
	high := loudest(spectrum(tone(12000, 48000, size, 0.8)))

	if low >= high {
		t.Errorf("200Hz landed in band %d and 12kHz in band %d, want the low one first", low, high)
	}
	if low > Bands/3 {
		t.Errorf("200Hz landed in band %d of %d, want it near the bottom", low, Bands)
	}
	if high < 2*Bands/3 {
		t.Errorf("12kHz landed in band %d of %d, want it near the top", high, Bands)
	}
}

func TestQuieterReadsLower(t *testing.T) {
	loud := spectrum(tone(1000, 48000, size, 0.9))
	soft := spectrum(tone(1000, 48000, size, 0.05))

	at := loudest(loud)
	if soft[at] >= loud[at] {
		t.Errorf("a quiet tone reads %v against a loud one's %v, want less", soft[at], loud[at])
	}
	if loud[at] <= 0 {
		t.Error("a loud tone reads as nothing")
	}
}

func TestBandsStayInRange(t *testing.T) {
	for _, at := range []struct {
		what    string
		samples []int16
	}{
		{"silence", make([]int16, size)},
		{"a full scale tone", tone(1000, 48000, size, 1)},
		{"a short period", tone(1000, 48000, 64, 1)},
		{"nothing at all", nil},
	} {
		for i, v := range spectrum(at.samples) {
			if v < 0 || v > 1 {
				t.Errorf("%s: band %d is %v, want 0 to 1", at.what, i, v)
			}
		}
	}
}
