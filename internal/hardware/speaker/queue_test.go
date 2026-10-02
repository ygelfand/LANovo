package speaker

import (
	"math"
	"testing"

	"github.com/ygelfand/LANovo/internal/board"
)

func TestMixSumsIntoWhatIsQueued(t *testing.T) {
	into := []int16{100, 200, 300}

	if got := mix(into, []int16{10, 20, 30}); got[0] != 110 || got[1] != 220 || got[2] != 330 {
		t.Errorf("mixed to %v, want [110 220 330]", got)
	}
}

func TestMixExtendsPastTheQueue(t *testing.T) {
	got := mix([]int16{100}, []int16{10, 20, 30})

	if len(got) != 3 {
		t.Fatalf("mixed to %d samples, want 3", len(got))
	}
	if got[0] != 110 || got[1] != 20 || got[2] != 30 {
		t.Errorf("mixed to %v, want [110 20 30]", got)
	}
}

// Two things at once are louder than either, and wrapping would turn that into a crack.
func TestMixClampsInsteadOfWrapping(t *testing.T) {
	got := mix([]int16{math.MaxInt16}, []int16{math.MaxInt16})
	if got[0] != math.MaxInt16 {
		t.Errorf("mixed to %d, want %d", got[0], math.MaxInt16)
	}

	got = mix([]int16{math.MinInt16}, []int16{math.MinInt16})
	if got[0] != math.MinInt16 {
		t.Errorf("mixed to %d, want %d", got[0], math.MinInt16)
	}
}

func TestMixIntoNothing(t *testing.T) {
	got := mix(nil, []int16{1, 2})
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("mixed to %v, want [1 2]", got)
	}
}

// A buffer the queue only part fills has silence spliced into it. Expected at the end of a sound;
// repeatedly during one means audio is arriving slower than it plays out.
func TestAPartFilledBufferCountsAsASplice(t *testing.T) {
	s := &Speaker{}
	s.pending = make([]int16, period) // half a buffer

	buf := make([]byte, period*Channels*Bits/8)
	s.fill(buf)

	if _, splices, _, _ := s.Stats(); splices != 1 {
		t.Errorf("spliced %d times, want 1", splices)
	}
}

// A full buffer is not a splice, however little is left behind it.
func TestAFullBufferIsNotASplice(t *testing.T) {
	s := &Speaker{}
	s.pending = make([]int16, period*Channels)

	buf := make([]byte, period*Channels*Bits/8)
	s.fill(buf)

	if _, splices, _, _ := s.Stats(); splices != 0 {
		t.Errorf("spliced %d times on a full buffer, want 0", splices)
	}
}

// Silence while nothing is playing is the resting state, not a fault. Only silence that interrupts
// something is worth counting.
func TestSilenceIsOnlyAnUnderrunAfterAudio(t *testing.T) {
	s := &Speaker{}
	buf := make([]byte, period*Channels*Bits/8)

	s.fill(buf)
	if _, _, underruns, _ := s.Stats(); underruns != 0 {
		t.Errorf("an idle speaker reported %d underruns", underruns)
	}

	s.pending = make([]int16, period*Channels)
	s.fill(buf)
	s.fill(buf)

	if _, _, underruns, _ := s.Stats(); underruns != 1 {
		t.Errorf("underran %d times, want 1", underruns)
	}
}

// Audio offered with no card is dropped rather than queued: nothing would drain it, so it would
// grow for as long as the speaker stayed away.
func TestAudioWithNoCardIsDroppedRatherThanQueued(t *testing.T) {
	s := &Speaker{}
	s.Play(make([]int16, 128))

	if s.Queued() != 0 {
		t.Errorf("%d frames queued with no card open", s.Queued())
	}
	if _, _, _, dropped := s.Stats(); dropped != 1 {
		t.Errorf("dropped %d, want 1", dropped)
	}
}

func TestVolumeScalesWhatIsAlreadyQueued(t *testing.T) {
	s := &Speaker{}
	s.pending = []int16{1000, 1000}
	s.SetVolume(0.5)

	buf := make([]byte, period*Channels*Bits/8)
	s.fill(buf)

	if got := int16(uint16(buf[0]) | uint16(buf[1])<<8); got != 500 {
		t.Errorf("first sample played at %d, want 500", got)
	}
	if got := s.mono[0]; got != 500 {
		t.Errorf("echo reference at %d, want 500", got)
	}
}

func TestAHardwareVolumeLeavesTheSamplesAndScalesOnlyTheEchoReference(t *testing.T) {
	board.Set(board.Ivy)
	defer board.Set(board.Blueberry)

	s := &Speaker{}
	s.pending = []int16{1000, 600}
	s.SetVolume(0.5)

	buf := make([]byte, period*Channels*Bits/8)
	s.fill(buf)

	left := int16(uint16(buf[0]) | uint16(buf[1])<<8)
	right := int16(uint16(buf[2]) | uint16(buf[3])<<8)
	if left != 1000 || right != 600 {
		t.Errorf("played %d, %d, want 1000, 600 in stereo at full scale", left, right)
	}
	if got := s.mono[0]; got != 400 {
		t.Errorf("echo reference at %d, want 400", got)
	}
}

// A speaker nobody has set is silent. Guessing full is how a device ends up shouting at someone.
func TestVolumeStartsSilent(t *testing.T) {
	if got := (&Speaker{}).Volume(); got != 0 {
		t.Errorf("a fresh speaker is at %v, want 0", got)
	}
}

// Silence is a level somebody can ask for, and must not read back as anything else.
func TestSilenceIsALevel(t *testing.T) {
	s := &Speaker{}
	s.SetVolume(1)
	s.SetVolume(0)

	if got := s.Volume(); got != 0 {
		t.Errorf("volume is %v after being set to nought, want 0", got)
	}
}

// A rest is silence of the right length, not a missing note: chimes rely on it for their spacing.
func TestARestIsSilenceOfTheRightLength(t *testing.T) {
	got := tone(Note{Freq: 0, Ms: 10}, 1)

	if want := Rate * 10 / 1000 * Channels; len(got) != want {
		t.Fatalf("a rest is %d samples, want %d", len(got), want)
	}
	for i, v := range got {
		if v != 0 {
			t.Fatalf("a rest has %d at sample %d", v, i)
		}
	}
}

// Both channels carry the same tone: a chime on one side only reads as a fault in the speaker.
func TestAToneIsOnBothChannels(t *testing.T) {
	got := tone(Note{Freq: 440, Ms: 50}, 1)

	var heard bool
	for i := 0; i+1 < len(got); i += Channels {
		if got[i] != got[i+1] {
			t.Fatalf("channels differ at frame %d: %d and %d", i/Channels, got[i], got[i+1])
		}
		if got[i] != 0 {
			heard = true
		}
	}
	if !heard {
		t.Error("the tone is silent")
	}
}

// The ramp is what stops the ends clicking. It reaches full only in the middle, so the ends are
// near silence rather than at it — a hundredth of the peak is quiet enough not to be heard as an
// edge, and asking for exactly zero would only be asserting where the envelope happens to land.
func TestAToneRampsInAndOut(t *testing.T) {
	got := tone(Note{Freq: 440, Ms: 100}, 1)

	var peak int16
	for _, v := range got {
		if v > peak {
			peak = v
		}
	}
	if peak == 0 {
		t.Fatal("the tone is silent")
	}

	quiet := peak / 100
	if first := got[0]; first > quiet || first < -quiet {
		t.Errorf("the tone starts at %d, which is not quiet against a peak of %d", first, peak)
	}
	if last := got[len(got)-1]; last > quiet || last < -quiet {
		t.Errorf("the tone ends at %d, which is not quiet against a peak of %d", last, peak)
	}
}
