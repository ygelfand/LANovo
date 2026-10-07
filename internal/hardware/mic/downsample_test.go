package mic

import (
	"math"
	"testing"
)

func stereoTone(hz, amp float64, frames int, left, right bool) []int16 {
	out := make([]int16, frames*Channels)
	for i := range frames {
		v := int16(amp * math.Sin(2*math.Pi*hz*float64(i)/Rate))
		if left {
			out[i*Channels] = v
		}
		if right {
			out[i*Channels+1] = v
		}
	}
	return out
}

func rmsOf(s []int16) float64 {
	var sum float64
	for _, v := range s {
		sum += float64(v) * float64(v)
	}
	return math.Sqrt(sum / float64(len(s)))
}

type pair struct{ l, r Downsampler }

func (p *pair) Mono16k(in []int16) []int16 {
	return mono(p.l.Channel16k(in, 0), p.r.Channel16k(in, 1))
}

func settled(hz float64, left, right bool) float64 {
	var d pair
	d.Mono16k(stereoTone(hz, 10000, Rate/10, left, right))
	return rmsOf(d.Mono16k(stereoTone(hz, 10000, Rate/10, left, right)))
}

func TestAPeriodGivesAThirdAsManySamples(t *testing.T) {
	if Decimation != 3 {
		t.Fatalf("decimation is %d, want 48k to 16k", Decimation)
	}
	var d pair
	if got := d.Mono16k(make([]int16, period*Channels)); len(got) != period/Decimation {
		t.Errorf("a period gave %d samples, want %d", len(got), period/Decimation)
	}
}

func TestSpeechFrequenciesPassAtTheirLevel(t *testing.T) {
	want := 10000 / math.Sqrt2
	for _, hz := range []float64{300, 1000, 3000, 5000} {
		if got := settled(hz, true, true); math.Abs(20*math.Log10(got/want)) > 1 {
			t.Errorf("%v Hz came out at %.1f dB", hz, 20*math.Log10(got/want))
		}
	}
}

func TestWhatWouldFoldBackIsRemoved(t *testing.T) {
	want := 10000 / math.Sqrt2
	for _, hz := range []float64{9000, 12000, 15000, 20000} {
		if got := settled(hz, true, true); 20*math.Log10(got/want+1e-9) > -50 {
			t.Errorf(
				"%v Hz, which would fold into the voice band, came through at %.1f dB",
				hz,
				20*math.Log10(got/want),
			)
		}
	}
}

func TestOneMicrophoneAloneIsStillHeard(t *testing.T) {
	both, one := settled(1000, true, true), settled(1000, false, true)
	if math.Abs(one-both/2) > both*0.02 {
		t.Errorf("one microphone came out at %.0f, want half of both, %.0f", one, both/2)
	}
}

func TestPeriodEdgesDoNotClick(t *testing.T) {
	var whole, split pair
	in := stereoTone(1000, 10000, 4800, true, true)
	a := whole.Mono16k(in)
	b := append(split.Mono16k(in[:1234*Channels]), split.Mono16k(in[1234*Channels:])...)
	if len(a) != len(b) {
		t.Fatalf("%d samples in one go, %d split", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("sample %d differs across a period edge: %d vs %d", i, a[i], b[i])
		}
	}
}

func TestFullScaleDoesNotWrap(t *testing.T) {
	var d pair
	in := make([]int16, 600)
	for i := range in {
		in[i] = -32768
	}
	for _, s := range d.Mono16k(in)[40:] {
		if s > -32000 {
			t.Fatalf("negative full scale came out at %d", s)
		}
	}
}
