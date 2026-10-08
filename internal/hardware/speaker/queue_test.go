package speaker

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/board"
)

func TestAPartFilledBufferCountsAsASplice(t *testing.T) {
	s := &Speaker{}
	s.queue.Push(make([]int16, period))

	buf := make([]byte, period*Channels*Bits/8)
	s.fill(buf)

	if _, splices, _, _ := s.Stats(); splices != 1 {
		t.Errorf("spliced %d times, want 1", splices)
	}
}

func TestAFullBufferIsNotASplice(t *testing.T) {
	s := &Speaker{}
	s.queue.Push(make([]int16, period*Channels))

	buf := make([]byte, period*Channels*Bits/8)
	s.fill(buf)

	if _, splices, _, _ := s.Stats(); splices != 0 {
		t.Errorf("spliced %d times on a full buffer, want 0", splices)
	}
}

func TestSilenceIsOnlyAnUnderrunAfterAudio(t *testing.T) {
	s := &Speaker{}
	buf := make([]byte, period*Channels*Bits/8)

	s.fill(buf)
	if _, _, underruns, _ := s.Stats(); underruns != 0 {
		t.Errorf("an idle speaker reported %d underruns", underruns)
	}

	s.queue.Push(make([]int16, period*Channels))
	s.fill(buf)
	s.fill(buf)

	if _, _, underruns, _ := s.Stats(); underruns != 1 {
		t.Errorf("underran %d times, want 1", underruns)
	}
}

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
	s.queue.Push([]int16{1000, 1000})
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
	s.queue.Push([]int16{1000, 600})
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

func TestVolumeStartsSilent(t *testing.T) {
	if got := (&Speaker{}).Volume(); got != 0 {
		t.Errorf("a fresh speaker is at %v, want 0", got)
	}
}

func TestSilenceIsALevel(t *testing.T) {
	s := &Speaker{}
	s.SetVolume(1)
	s.SetVolume(0)

	if got := s.Volume(); got != 0 {
		t.Errorf("volume is %v after being set to nought, want 0", got)
	}
}
