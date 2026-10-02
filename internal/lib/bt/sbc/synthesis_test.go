package sbc

import (
	"encoding/binary"
	"os"
	"testing"
)

// The decoder against the one it borrowed its tables from.
//
// testdata/vectors holds an SBC stream and the same stream decoded by ffmpeg, which is where the
// prototype window and the matrices came from. Sample for sample is the only check worth making
// here: a filterbank with a coefficient slightly wrong produces audio that is quietly wrong, and
// nothing short of comparing the samples notices.
//
// Made with:
//
//	ffmpeg -f lavfi -i "sine=frequency=440:duration=0.12:sample_rate=44100,\
//	    aformat=channel_layouts=stereo" -c:a sbc tone.sbc
//	ffmpeg -i tone.sbc -f s16le -acodec pcm_s16le tone.pcm
//
// tone is stereo at 44100, mono the same at one channel, sweep 48000, and short carries twice as
// many frames of half the blocks (-sbc_delay 0.004).
//
// joint is not synthetic: it is what a phone actually sent, captured off the link. Joint stereo at
// bitpool 53, sixteen blocks, eight subbands — none of which the encoder above produces, and all of
// which the stereo bit allocation got wrong while every vector here passed.
//
// No vector for four subbands: ffmpeg's encoder only writes eight, so that path is held by the
// fuzzer and by the shape checks rather than against a reference. Nor for mSBC, which is a
// different syncword (0xad against 0x9c) and belongs to HFP rather than A2DP.

// reference reads a vector and the samples it should come out as, interleaved as ffmpeg writes
// them.
func reference(t *testing.T, name string) (coded []byte, want []int16) {
	t.Helper()

	coded, err := os.ReadFile("testdata/vectors/" + name + ".sbc")
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile("testdata/vectors/" + name + ".pcm")
	if err != nil {
		t.Fatal(err)
	}

	want = make([]int16, len(raw)/2)
	for i := range want {
		want[i] = int16(binary.LittleEndian.Uint16(raw[2*i:]))
	}
	return coded, want
}

func TestDecodingMatchesTheReference(t *testing.T) {
	for _, name := range []string{"tone", "mono", "sweep", "short", "joint"} {
		t.Run(name, func(t *testing.T) { matches(t, name) })
	}
}

func matches(t *testing.T, name string) {
	t.Helper()
	coded, want := reference(t, name)

	var f Filter
	var got []int16
	frames := 0

	for at := 0; at < len(coded); {
		start, ok := Find(coded[at:])
		if !ok {
			break
		}
		at += start

		fr, err := Unpack(coded[at:])
		if err != nil {
			t.Fatalf("frame %d at %d: %v", frames, at, err)
		}

		out, err := f.Synthesize(fr)
		if err != nil {
			t.Fatalf("frame %d: %v", frames, err)
		}

		// Interleaved, to line up with what ffmpeg wrote.
		for i := range out[0] {
			for ch := range out {
				got = append(got, out[ch][i])
			}
		}

		at += fr.Header.Length()
		frames++
	}

	if frames == 0 {
		t.Fatal("no frames were found in the vector")
	}
	t.Logf("%d frames, %d samples", frames, len(got))

	if len(got) != len(want) {
		t.Fatalf("decoded %d samples, the reference has %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample %d is %d, the reference says %d", i, got[i], want[i])
		}
	}
}

// The filterbank carries history between frames, and that is load bearing rather than an
// optimisation: each output sample is a window over the last ten blocks. A filter started fresh for
// every frame produces a different signal — a click at each boundary, 128 times a second.
func TestTheFilterbankCarriesHistoryBetweenFrames(t *testing.T) {
	coded, _ := reference(t, "tone")

	carried := decoded(t, coded, false)
	restarted := decoded(t, coded, true)

	if len(carried) != len(restarted) {
		t.Fatalf("%d samples against %d", len(carried), len(restarted))
	}

	same := 0
	for i := range carried {
		if carried[i] == restarted[i] {
			same++
		}
	}

	if same == len(carried) {
		t.Fatal("restarting the filterbank every frame changed nothing, so it carries no history")
	}
	t.Logf("%d of %d samples differ when the filterbank is restarted per frame",
		len(carried)-same, len(carried))
}

// decoded runs a stream through, either on one filter or on a fresh one per frame.
func decoded(t *testing.T, coded []byte, fresh bool) []int16 {
	t.Helper()

	var f Filter
	var out []int16

	for at := 0; at < len(coded); {
		start, ok := Find(coded[at:])
		if !ok {
			break
		}
		at += start

		fr, err := Unpack(coded[at:])
		if err != nil {
			break
		}

		if fresh {
			f = Filter{}
		}

		audio, err := f.Synthesize(fr)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, audio[0]...)

		at += fr.Header.Length()
	}
	return out
}
