package speaker

import (
	"math"
	"testing"
)

// at measures how much of the signal sits at hz, by correlating against a sine and cosine there.
func at(samples []int16, hz float64) float64 {
	var re, im float64
	for i := 0; i < len(samples); i += Channels {
		p := 2 * math.Pi * hz * float64(i/Channels) / Rate
		re += float64(samples[i]) * math.Cos(p)
		im += float64(samples[i]) * math.Sin(p)
	}
	n := float64(len(samples) / Channels)
	return math.Hypot(re, im) / n
}

// voice is a mono tone at the rate a pipeline sends.
func voice(hz float64, ms int) []int16 {
	out := make([]int16, VoiceRate*ms/1000)
	for i := range out {
		out[i] = int16(0.5 * math.MaxInt16 * math.Sin(2*math.Pi*hz*float64(i)/VoiceRate))
	}
	return out
}

func run(r Resampler, mono []int16) []int16 {
	return r.Run(mono, make([]int16, 0, len(mono)*VoiceUpsample*Channels))
}

// The reason the filter exists. Stretching 16 kHz to 48 kHz by repeating samples leaves a mirror of
// the signal at the input rate minus its frequency — for 3 kHz that is 13 kHz, plainly audible as
// grit. The filter is supposed to put that image far below the tone.
func TestTheFilterRejectsTheImageThatHoldingLeaves(t *testing.T) {
	const hz = 3000
	mono := voice(hz, 200)

	held := run(hold{}, mono)
	filtered := run(newSinc(), mono)

	image := Rate/VoiceUpsample - hz // 13 kHz

	heldRatio := at(held, float64(image)) / at(held, hz)
	sincRatio := at(filtered, float64(image)) / at(filtered, hz)

	if sincRatio >= heldRatio {
		t.Fatalf("the filter left as much image as holding: %.4f against %.4f", sincRatio, heldRatio)
	}
	if sincRatio > 0.01 {
		t.Errorf("the image is %.4f of the tone, want under 0.01", sincRatio)
	}
}

// Whatever the method, the tone itself has to survive it.
func TestEveryResamplerKeepsTheTone(t *testing.T) {
	const hz = 1000
	mono := voice(hz, 200)

	for _, r := range Resamplings() {
		t.Run(string(r), func(t *testing.T) {
			made, settled := NewResampler(r)
			if settled != r {
				t.Fatalf("asked for %s and got %s", r, settled)
			}

			out := run(made, mono)
			if want := len(mono) * VoiceUpsample * Channels; len(out) != want {
				t.Fatalf("%d samples out, want %d", len(out), want)
			}
			if got := at(out, hz); got < 0.2*math.MaxInt16 {
				t.Errorf("the tone came out at %.0f, which is most of the way to silence", got)
			}
		})
	}
}

// A reply arrives in chunks. Feeding it piecewise has to give the same signal as feeding it whole,
// or there is a seam at every boundary.
func TestChunksComeOutTheSameAsOnePiece(t *testing.T) {
	mono := voice(1000, 100)

	whole := run(newSinc(), mono)

	piecewise := make([]int16, 0, len(whole))
	chunked := newSinc()
	for i := 0; i < len(mono); i += 160 {
		piecewise = chunked.Run(mono[i:min(i+160, len(mono))], piecewise)
	}

	if len(piecewise) != len(whole) {
		t.Fatalf("chunked gave %d samples, whole gave %d", len(piecewise), len(whole))
	}
	for i := range whole {
		if piecewise[i] != whole[i] {
			t.Fatalf("chunked and whole differ at sample %d: %d against %d", i, piecewise[i], whole[i])
		}
	}
}

// Reset is what a barge-in does: the next utterance is unrelated, so none of the last one should
// bleed into its first samples.
func TestResetDropsTheTail(t *testing.T) {
	loud := voice(1000, 50)

	r := newSinc()
	run(r, loud)
	r.Reset()

	fresh := newSinc()
	quiet := make([]int16, 160)

	if got, want := run(r, quiet), run(fresh, quiet); len(got) != len(want) {
		t.Fatalf("%d samples against %d", len(got), len(want))
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("a reset resampler still carried the last utterance at sample %d", i)
			}
		}
	}
}

// Both channels carry the same voice: a reply on one side only reads as a broken speaker.
func TestVoiceLandsOnBothChannels(t *testing.T) {
	out := run(newSinc(), voice(1000, 20))

	for i := 0; i+1 < len(out); i += Channels {
		if out[i] != out[i+1] {
			t.Fatalf("channels differ at frame %d: %d and %d", i/Channels, out[i], out[i+1])
		}
	}
}

// An unknown name falls back to the filter rather than to silence or a panic.
func TestAnUnknownResamplerFallsBackToTheFilter(t *testing.T) {
	made, settled := NewResampler(Resampling("Bogus"))

	if settled != ResampleSinc {
		t.Errorf("settled on %s, want %s", settled, ResampleSinc)
	}
	if made == nil {
		t.Fatal("no resampler came back")
	}
}

// The sweep is the signal for comparing the options by ear, so it has to be at the pipeline's rate
// and quiet at both ends.
func TestTheSweepIsAVoiceSignalThatDoesNotClick(t *testing.T) {
	got := VoiceSweep()

	if want := VoiceRate * sweepMs / 1000; len(got) != want {
		t.Fatalf("the sweep is %d samples, want %d", len(got), want)
	}

	var peak int16
	for _, v := range got {
		if v > peak {
			peak = v
		}
	}
	if quiet := peak / 100; got[0] > quiet || got[len(got)-1] > quiet {
		t.Errorf("the sweep starts at %d and ends at %d against a peak of %d",
			got[0], got[len(got)-1], peak)
	}
}

// What each way of stretching a reply costs, since the whole point of the seam is being able to
// fall back if the filter is too dear for the part it runs on.
func BenchmarkResample(b *testing.B) {
	// A second of speech, which is the order a reply arrives in.
	mono := voice(1000, 1000)

	for _, r := range Resamplings() {
		b.Run(string(r), func(b *testing.B) {
			made, _ := NewResampler(r)
			out := make([]int16, 0, len(mono)*VoiceUpsample*Channels)

			for b.Loop() {
				made.Run(mono, out[:0])
			}
			b.SetBytes(int64(len(mono) * 2))
		})
	}
}

// The voice path's level, which folding it into Rational could have moved without anything failing.
//
// Each phase sums to the headroom and they all sum to the same thing: a reply comes out at the
// level it went in, less the headroom, and no phase is louder than its neighbours. The filter this
// replaced summed to 0.89826, 0.89686 and 0.89826 — 0.03 dB quieter overall and 0.014 dB of ripple
// between the three, both far under anything audible, which is why this was a fold and not a
// retune.
func TestTheVoiceFilterKeepsItsLevel(t *testing.T) {
	up, _ := NewResampler(ResampleSinc)

	s, ok := up.(*sinc)
	if !ok {
		t.Fatalf("the sinc resampler is a %T", up)
	}

	phases, taps := s.up.up, ratioTaps
	for p := range phases {
		var sum float32
		for k := range taps {
			sum += s.up.filter[p+k*phases]
		}
		if diff := sum - voiceHeadroom; diff > 0.001 || diff < -0.001 {
			t.Errorf("phase %d sums to %.5f, want %.2f", p, sum, voiceHeadroom)
		}
	}
}
