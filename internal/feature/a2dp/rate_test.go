package a2dp

import (
fixtures "github.com/ygelfand/libcountertop/pkg/bluetooth/sbc/testdata"
	"testing"

	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/libcountertop/pkg/bluetooth/sbc"
)

// The card runs at one rate and everything handed to it has to arrive at that rate. A phone picks
// 44.1 for SBC, and 44100 samples a second into a card clocked at 48000 is a semitone and a half
// sharp with the queue starving besides.
//
// The frames are the ones internal/lib/bt/sbc checks its filterbank against, so this measures the
// join rather than a reconstruction of it: real coded bytes in, what the card is handed out. sweep
// is already at the card's rate and so covers the other branch, where nothing should be filtered.

func TestThePlayPathFeedsTheCardAtItsOwnRate(t *testing.T) {
	for _, name := range []string{"tone", "mono", "sweep"} {
		t.Run(name, func(t *testing.T) {
			coded, err := fixtures.Read(name+".sbc")
			if err != nil {
				t.Fatal(err)
			}

			// What the phone chose, read off the stream itself.
			start, ok := sbc.Find(coded)
			if !ok {
				t.Fatal("no frame in the vector")
			}
			first, err := sbc.Unpack(coded[start:])
			if err != nil {
				t.Fatal(err)
			}
			chose := first.Header.Rate

			s := &Sink{}
			s.start("rate test")
			s.resample = speaker.NewRational(chose, speaker.Rate, 2)

			// The counters reset every time the stream reports itself, and it reports on the very
			// first frame. That frame is also where the filterbank and the resampler fill, so it
			// serves as the warm up and everything after it is what gets measured.
			var in, frames int
			for at := start; at < len(coded); {
				skip, ok := sbc.Find(coded[at:])
				if !ok {
					break
				}
				at += skip

				fr, err := sbc.Unpack(coded[at:])
				if err != nil {
					t.Fatalf("frame at %d: %v", at, err)
				}
				if fr.Header.Rate != chose {
					t.Fatalf("the rate changed mid stream, %d to %d", chose, fr.Header.Rate)
				}

				s.play(coded[at : at+fr.Header.Length()])
				at += fr.Header.Length()

				if frames++; frames > 1 {
					in += fr.Header.Blocks * fr.Header.Subbands
				} else {
					s.frames, s.bad, s.samples = 0, 0, 0
				}
			}

			if frames < 2 {
				t.Fatalf("%d frames in the vector, which measures nothing", frames)
			}
			if s.bad != 0 {
				t.Fatalf("%d of %d frames would not decode: %v", s.bad, s.frames, s.why)
			}

			// Whole samples do not divide 160:147, so the count wanders by one either way.
			want := in * speaker.Rate / chose
			if s.samples < want-2 || s.samples > want+2 {
				t.Errorf("%d samples to the card from %d at %d, want %d",
					s.samples, in, chose, want)
			}
		})
	}
}

// A stream the card can already play must pass through untouched rather than be filtered.
func TestAStreamAtTheCardsRateIsUntouched(t *testing.T) {
	r := speaker.NewRational(speaker.Rate, speaker.Rate, 2)

	if up, down := r.Ratio(); up != 1 || down != 1 {
		t.Fatalf("the card's own rate reduced to %d:%d", up, down)
	}
}
