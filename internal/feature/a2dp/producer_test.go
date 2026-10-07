package a2dp

import (
fixtures "github.com/ygelfand/libcountertop/pkg/bluetooth/sbc/testdata"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/libcountertop/pkg/bluetooth/sbc"
)

// What the arbiter asks of this, and what happens to audio while something else has the speaker.

// fed plays one frame from a vector and reports how many samples reached the card.
func fed(t *testing.T, s *Sink) int {
	t.Helper()

	coded, err := fixtures.Read("tone.sbc")
	if err != nil {
		t.Fatal(err)
	}

	at, ok := sbc.Find(coded)
	if !ok {
		t.Fatal("no frame in the vector")
	}
	fr, err := sbc.Unpack(coded[at:])
	if err != nil {
		t.Fatal(err)
	}

	// The counters reset each time the stream reports itself, and it reports on the first frame.
	s.said = time.Now()

	before := s.samples
	s.play(coded[at : at+fr.Header.Length()])
	return s.samples - before
}

// Audio arriving while stood down is dropped rather than queued. A phone goes on sending whatever
// this device does with it, so keeping it would play minutes late.
func TestNothingIsPlayedWhileStoodDown(t *testing.T) {
	s := &Sink{}
	s.start("stand test")
	s.resample = speaker.NewRational(44100, speaker.Rate, 2)

	if got := fed(t, s); got == 0 {
		t.Fatal("nothing reached the card while standing")
	}

	s.Stand(true)
	if got := fed(t, s); got != 0 {
		t.Errorf("%d samples reached the card while stood down", got)
	}

	s.Stand(false)
	if got := fed(t, s); got == 0 {
		t.Error("nothing reached the card after standing back up")
	}
}

// The arbiter delivers a standing again whenever anything moves, so the same one twice has to be
// the same as once. Pausing the phone on every repeat would fight a person pressing play.
func TestTheSameStandingTwiceIsOnce(t *testing.T) {
	s := &Sink{}
	s.start("repeat test")

	s.Stand(true)
	s.Stand(true)
	s.Stand(true)

	s.mu.Lock()
	down := s.down
	s.mu.Unlock()

	if !down {
		t.Error("a repeated standing lost the standing")
	}

	// And back up, which must not leave it thinking it is still down.
	s.Stand(false)
	s.Stand(false)

	s.mu.Lock()
	down = s.down
	s.mu.Unlock()

	if down {
		t.Error("standing back up twice left it down")
	}
}

// Ducking scales what is written next rather than silencing it: a voice turn talks over music, it
// does not replace it.
func TestDuckingQuietensRatherThanStops(t *testing.T) {
	s := &Sink{}
	s.start("duck test")
	s.resample = speaker.NewRational(44100, speaker.Rate, 2)

	s.Duck(true)
	if got := fed(t, s); got == 0 {
		t.Error("ducking stopped the audio instead of quietening it")
	}

	s.mu.Lock()
	gain := s.gain
	s.mu.Unlock()

	if gain != ducked {
		t.Errorf("ducked to %v, want %v", gain, ducked)
	}

	s.Duck(false)

	s.mu.Lock()
	gain = s.gain
	s.mu.Unlock()

	if gain != 1 {
		t.Errorf("unducked to %v, want 1", gain)
	}
}
