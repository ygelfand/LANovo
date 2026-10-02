package noise

import "github.com/ygelfand/LANovo/internal/hardware/speaker"

var _ speaker.Producer = (*Machine)(nil)

// Stand implements speaker.Producer. Generated sound has no position to keep, so what was queued is
// thrown away rather than put aside: picking up again means generating the next chunk, and holding
// a second of sound from before the interruption would only play it late.
func (m *Machine) Stand(down bool) {
	m.mu.Lock()
	first := down && !m.down
	m.down = down
	m.mu.Unlock()

	// Only on the way down: the arbiter says so before whatever displaced this queues anything, so
	// what is in there now is still this machine's to drop.
	if first {
		speaker.Get().Take()
	}
}

// Duck implements speaker.Producer: it sets the level for the chunks generated next.
func (m *Machine) Duck(on bool) {
	gain := float32(1)
	if on {
		gain = ducked
	}

	m.mu.Lock()
	m.gain = gain
	m.mu.Unlock()
}

// Requeue implements speaker.Producer. There is nothing to rescale: what is queued is at most half a
// second, and the chunks after it are generated at the new level anyway.
func (m *Machine) Requeue() {}
