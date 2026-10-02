package speaker

import (
	"math"
	"math/rand"
	"testing"
)

// 44100 to 48000 is 160 to 147 once reduced, and reducing it is what makes the filter affordable:
// unreduced it would be 48000 phases.
func TestTheRatioReduces(t *testing.T) {
	for _, c := range []struct {
		from, to int
		up, down int
	}{
		{44100, 48000, 160, 147},
		{48000, 44100, 147, 160},
		{16000, 48000, 3, 1},
		{48000, 48000, 1, 1},
		{22050, 48000, 320, 147},
	} {
		up, down := NewRational(c.from, c.to, 2).Ratio()
		if up != c.up || down != c.down {
			t.Errorf("%d to %d reduced to %d:%d, want %d:%d", c.from, c.to, up, down, c.up, c.down)
		}
	}
}

// A card already at the source rate must not be filtered: the samples are right as they are, and
// running them through anyway costs sixteen multiplies each to make them slightly worse.
func TestMatchingRatesPassStraightThrough(t *testing.T) {
	in := sine(48000, 1000, 480, 2)

	got := NewRational(48000, 48000, 2).Run(in)
	if len(got) != len(in) {
		t.Fatalf("%d samples out of %d in", len(got), len(in))
	}
	for i := range in {
		if got[i] != in[i] {
			t.Fatalf("sample %d came back as %d, want %d", i, got[i], in[i])
		}
	}
}

// The point of the thing: a second of 44.1 has to come out as a second of 48, or the audio drifts
// against the clock and the card either starves or overruns.
func TestASecondComesOutASecond(t *testing.T) {
	const frames = 44100

	got := NewRational(44100, 48000, 2).Run(sine(44100, 1000, frames, 2))

	// All of it. A stream starts against silence rather than against nothing, so the first input
	// sample already has the history the filter needs and none of the second is swallowed.
	want := frames * 160 / 147

	if out := len(got) / 2; out != want {
		t.Errorf("%d frames out of %d, want %d", out, frames, want)
	}
}

// A stream begins where its first sample does. Filling the history costs ratioTaps-1 frames, and
// taking them out of the front of the audio is a third of a millisecond of every utterance missing.
func TestNothingIsSwallowedAtTheStart(t *testing.T) {
	r := NewRational(16000, 48000, 1)

	// One sample, loud, with nothing before it.
	got := r.Run([]int16{10000})

	if len(got) != 3 {
		t.Fatalf("%d samples out of one, want three", len(got))
	}
	for i, v := range got {
		if v == 0 {
			t.Errorf("sample %d of the first input came out silent", i)
		}
	}
}

// A tone has to come out the same tone at the same level. Getting the phase layout backwards still
// produces plausible audio, just with the band mirrored, so this measures the tone rather than
// eyeballing the samples.
func TestAToneKeepsItsPitchAndItsLevel(t *testing.T) {
	const (
		hz   = 1000
		peak = 10000
	)

	// Half a second, which is long enough that the startup transient does not move the measurement.
	in := sine(44100, hz, 22050, 2)
	got := NewRational(44100, 48000, 2).Run(in)

	// Skip the first millisecond: the filter is still filling and those samples are legitimately low.
	got = got[48*2:]

	amp := level(got, 0, 2, 48000, hz)
	if math.Abs(amp-peak)/peak > 0.02 {
		t.Errorf("the tone came out at %.0f, want about %d", amp, peak)
	}

	// Everything that is not the tone is the filter's leftovers, and there should be very little of
	// it. A phase layout read backwards puts an image at the input rate minus the tone.
	if image := level(got, 0, 2, 48000, 44100-hz); image > peak*0.01 {
		t.Errorf("an image at %d Hz came out at %.0f", 44100-hz, image)
	}

	rest := math.Sqrt(math.Max(power(got, 0, 2)-amp*amp/2, 0))
	if rest > peak*0.02 {
		t.Errorf("%.0f rms of everything that is not the tone, against a tone of %d", rest, peak)
	}
}

// Audio arrives as whatever an SBC frame happened to hold, so an output sample near the start of a
// buffer is made partly from the end of the one before. Restarting per buffer clicks at every seam.
func TestBuffersJoinUpWhereTheyMeet(t *testing.T) {
	in := sine(44100, 1000, 8192, 2)

	whole := NewRational(44100, 48000, 2).Run(append([]int16(nil), in...))

	// The same audio handed over in the uneven chunks a stream actually delivers.
	chunked := &Rational{}
	*chunked = *NewRational(44100, 48000, 2)

	var pieces []int16
	rng := rand.New(rand.NewSource(1))
	for at := 0; at < len(in); {
		n := (1 + rng.Intn(128)) * 2
		n = min(n, len(in)-at)

		pieces = append(pieces, chunked.Run(in[at:at+n])...)
		at += n
	}

	if len(pieces) != len(whole) {
		t.Fatalf("%d samples in pieces against %d in one go", len(pieces), len(whole))
	}
	for i := range whole {
		if pieces[i] != whole[i] {
			t.Fatalf("sample %d is %d in pieces and %d in one go", i, pieces[i], whole[i])
		}
	}
}

// Reset is for a stream that stopped. Without it the next one starts part way through the last, so
// a pause and a resume splice the two together.
func TestResetStartsOver(t *testing.T) {
	in := sine(44100, 1000, 2048, 2)

	r := NewRational(44100, 48000, 2)
	first := append([]int16(nil), r.Run(in)...)

	r.Reset()
	second := r.Run(in)

	if len(first) != len(second) {
		t.Fatalf("%d samples then %d from the same input", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("sample %d is %d after a reset, want %d", i, second[i], first[i])
		}
	}
}

// Left and right are filtered apart. Reading the interleave wrong mixes them, which on a stereo
// recording sounds like it is working right up until something is panned.
func TestTheEarsStaySeparate(t *testing.T) {
	const frames = 4096

	// A tone in one ear and silence in the other.
	in := make([]int16, frames*2)
	for i := range frames {
		in[i*2] = int16(10000 * math.Sin(2*math.Pi*1000*float64(i)/44100))
	}

	got := NewRational(44100, 48000, 2).Run(in)

	if left := level(got[96:], 0, 2, 48000, 1000); left < 9000 {
		t.Errorf("the tone came out of the left at %.0f, want about 10000", left)
	}
	if right := math.Sqrt(power(got[96:], 1, 2)); right > 100 {
		t.Errorf("%.0f rms leaked into the silent ear", right)
	}
}

// Nothing in is nothing out, and a resampler that has never been given a full filter's worth of
// history yet still has to answer rather than reach behind the start of the buffer.
func TestTooLittleToWorkWith(t *testing.T) {
	r := NewRational(44100, 48000, 2)

	if got := r.Run(nil); len(got) != 0 {
		t.Errorf("%d samples out of nothing", len(got))
	}

	// One frame at a time, from cold. The first several produce nothing at all.
	in := sine(44100, 1000, 64, 2)
	var out []int16
	for at := 0; at < len(in); at += 2 {
		out = append(out, r.Run(in[at:at+2])...)
	}

	// 64 frames in, minus the taps the filter needs before it can answer, scaled up.
	if want := (64 - ratioTaps) * 160 / 147; len(out)/2 < want {
		t.Errorf("%d frames from 64 fed one at a time, want at least %d", len(out)/2, want)
	}
}

// Full scale in must not wrap. A sample that wraps is full scale of the opposite sign, which is a
// crack rather than a loud moment.
func TestFullScaleSaturates(t *testing.T) {
	// A square wave at full scale, which is the worst case: the filter overshoots every edge.
	in := make([]int16, 4096)
	for i := range len(in) / 2 {
		v := int16(math.MaxInt16)
		if (i/16)%2 == 1 {
			v = math.MinInt16
		}
		in[i*2], in[i*2+1] = v, v
	}

	got := NewRational(44100, 48000, 2).Run(in)

	// The filter rings about nine percent past an edge, so full scale in must pin the output rather
	// than wrap it. Pinned samples are the evidence saturation ran at all.
	var pinned int
	for _, v := range got {
		if v == math.MaxInt16 || v == math.MinInt16 {
			pinned++
		}
	}
	if pinned == 0 {
		t.Fatal("a full scale square came back with nothing at full scale")
	}
}

// What pinning is, at the two values that matter. Wrapping here is a full scale crack.
func TestClipSaturates(t *testing.T) {
	for _, c := range []struct {
		in   float32
		want int16
	}{
		{0, 0},
		{32766.6, 32766},
		{32767, 32767},
		{40000, math.MaxInt16},
		{1e9, math.MaxInt16},
		{-32768, math.MinInt16},
		{-40000, math.MinInt16},
		{-1e9, math.MinInt16},
	} {
		if got := clip(c.in); got != c.want {
			t.Errorf("clip(%v) is %d, want %d", c.in, got, c.want)
		}
	}
}

// sine is interleaved channels all carrying the same tone.
func sine(rate int, hz float64, frames, channels int) []int16 {
	out := make([]int16, frames*channels)
	for i := range frames {
		v := int16(10000 * math.Sin(2*math.Pi*hz*float64(i)/float64(rate)))
		for c := range channels {
			out[i*channels+c] = v
		}
	}
	return out
}

// level is the amplitude of one frequency in one channel, by correlating against it.
func level(x []int16, ch, channels, rate int, hz float64) float64 {
	n := len(x) / channels

	var re, im float64
	for i := range n {
		w := 2 * math.Pi * hz * float64(i) / float64(rate)
		v := float64(x[i*channels+ch])

		re += v * math.Cos(w)
		im -= v * math.Sin(w)
	}
	return 2 * math.Hypot(re, im) / float64(n)
}

// power is the mean square of one channel.
func power(x []int16, ch, channels int) float64 {
	n := len(x) / channels

	var sum float64
	for i := range n {
		v := float64(x[i*channels+ch])
		sum += v * v
	}
	return sum / float64(n)
}
