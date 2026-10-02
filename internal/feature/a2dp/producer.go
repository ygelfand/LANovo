package a2dp

import (
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/lib/bt/avrcp"
)

var _ speaker.Producer = (*Sink)(nil)

// ducked is the multiplier applied under a voice turn.
const ducked = 0.2

// Stand implements speaker.Producer, pausing the phone rather than muting it.
func (s *Sink) Stand(down bool) {
	s.mu.Lock()
	first := down && !s.down
	s.down = down
	if first {
		s.drift.Reset()
	}
	s.mu.Unlock()

	if !first {
		return
	}

	speaker.Get().Drain()
	s.press(avrcp.OpPause)
}

// Duck implements speaker.Producer, setting the level for what is written next.
func (s *Sink) Duck(on bool) {
	gain := float32(1)
	if on {
		gain = ducked
	}

	s.mu.Lock()
	s.gain = gain
	s.mu.Unlock()
}

// Requeue implements speaker.Producer, scaling what is already queued.
func (s *Sink) Requeue() {
	s.mu.Lock()
	gain := s.gain
	s.mu.Unlock()

	if gain == 1 {
		return
	}
	speaker.Get().Adjust(func(samples []int16) { speaker.Scale(samples, gain) })
}
