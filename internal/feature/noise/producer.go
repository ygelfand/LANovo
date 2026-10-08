package noise

import (
	"github.com/ygelfand/libcountertop/pkg/audio/background"

	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

var _ background.Producer = (*Machine)(nil)

func (m *Machine) Stand(down bool) {
	m.mu.Lock()
	first := down && !m.down
	m.down = down
	m.mu.Unlock()

	if first {
		speaker.Get().Take()
	}
}

func (m *Machine) Duck(on bool) {
	gain := float32(1)
	if on {
		gain = ducked
	}

	m.mu.Lock()
	m.gain = gain
	m.mu.Unlock()
}

func (m *Machine) Requeue() {}
