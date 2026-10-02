package analysis

import (
	"math"
	"testing"
)

func tone(hz float64, rate, n int, amp float64) []int16 {
	out := make([]int16, n)
	for i := range out {
		out[i] = int16(amp * math.MaxInt16 * math.Sin(2*math.Pi*hz*float64(i)/float64(rate)))
	}
	return out
}

func loudest(a Analysis) int {
	at := 0
	for b := range a.Bands {
		if a.Bands[b] > a.Bands[at] {
			at = b
		}
	}
	return at
}

func TestAToneLandsInTheSameBandAtEitherRate(t *testing.T) {
	for _, hz := range []float64{200, 1000, 4000} {
		var bands []int
		for _, rate := range []int{16000, 48000} {
			a := New(rate)
			a.Write(tone(hz, rate, rate/2, 0.5))
			bands = append(bands, loudest(a.Latest()))
		}
		if d := bands[0] - bands[1]; d < -1 || d > 1 {
			t.Errorf("%v Hz lands in band %d at 16 kHz and %d at 48 kHz", hz, bands[0], bands[1])
		}
	}
}

func TestBandsRiseWithPitch(t *testing.T) {
	a := New(48000)
	last := -1
	for _, hz := range []float64{100, 400, 1600, 6400} {
		a.Reset()
		a.Write(tone(hz, 48000, 24000, 0.5))
		b := loudest(a.Latest())
		if b <= last {
			t.Errorf("%v Hz in band %d, not above %d", hz, b, last)
		}
		last = b
	}
}

func TestSilenceIsFlat(t *testing.T) {
	a := New(16000)
	a.Write(make([]int16, 16000))
	got := a.Latest()
	if got.Level != 0 || got.Peak != 0 || got.Onsets != 0 {
		t.Errorf("silence gave level %v peak %v onsets %d", got.Level, got.Peak, got.Onsets)
	}
	for b, v := range got.Bands {
		if v != 0 {
			t.Errorf("silence lit band %d at %v", b, v)
		}
	}
}

func TestEachSyllableIsOneOnset(t *testing.T) {
	const rate = 16000
	a := New(rate)
	var in []int16
	for range 4 {
		in = append(in, tone(300, rate, rate/5, 0.5)...)
		in = append(in, make([]int16, rate*3/10)...)
	}
	a.Write(in)
	if got := a.Latest().Onsets; got != 4 {
		t.Errorf("four bursts gave %d onsets", got)
	}
}

func TestASteadyToneIsOneOnset(t *testing.T) {
	a := New(16000)
	a.Write(tone(300, 16000, 32000, 0.5))
	if got := a.Latest().Onsets; got != 1 {
		t.Errorf("two seconds of one tone gave %d onsets", got)
	}
}

func TestLevelFollowsLoudness(t *testing.T) {
	a := New(16000)
	a.Write(tone(300, 16000, 8000, 0.1))
	quiet := a.Latest().Level
	a.Write(tone(300, 16000, 8000, 0.8))
	loud := a.Latest().Level
	if !(quiet > 0 && loud > quiet*4) {
		t.Errorf("level %v at 0.1 and %v at 0.8", quiet, loud)
	}
}

func TestWriteDoesNotAllocate(t *testing.T) {
	a := New(48000)
	in := tone(440, 48000, 4800, 0.5)
	if n := testing.AllocsPerRun(10, func() { a.Write(in) }); n != 0 {
		t.Errorf("Write allocated %v times", n)
	}
}

func TestRingDropsRatherThanGrows(t *testing.T) {
	r := NewRing(10)
	r.Offer(make([]int16, 8))
	r.Offer(make([]int16, 8))
	if got := len(r.Drain(nil)); got != 10 {
		t.Errorf("drained %d, want the ring's 10", got)
	}
	if r.Dropped() != 6 {
		t.Errorf("dropped %d, want 6", r.Dropped())
	}
}

func TestRingOfferDoesNotWaitOnAReader(t *testing.T) {
	r := NewRing(10)
	r.mu.Lock()
	r.Offer(make([]int16, 4))
	r.mu.Unlock()
	if got := len(r.Drain(nil)); got != 0 {
		t.Errorf("an offer made while locked landed %d samples", got)
	}
}

func TestMonoAverages(t *testing.T) {
	got := Mono([]int16{100, 300, -200, 200}, 2, nil)
	if len(got) != 2 || got[0] != 200 || got[1] != 0 {
		t.Errorf("mono of two frames = %v", got)
	}
}
