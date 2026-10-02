package media

import "github.com/ygelfand/LANovo/internal/hardware/speaker"

var _ speaker.Producer = (*Player)(nil)

// ducked is the multiplier applied under a voice turn.
const ducked = 0.2

// Stand implements speaker.Producer. The same standing twice is the arbiter repeating itself.
func (p *Player) Stand(down bool) {
	p.mu.Lock()
	first := down && !p.down
	p.down = down
	kept := p.kept
	if !down {
		p.kept = nil
	}
	p.mu.Unlock()

	// Only on the way down, while the queue is still this player's.
	if first {
		aside := speaker.Get().Take()

		p.mu.Lock()
		p.kept = append(p.kept, aside...)
		p.mu.Unlock()
		return
	}

	if !down && len(kept) > 0 {
		speaker.Get().Play(kept)
	}
}

// Duck implements speaker.Producer. It sets the level for what is written next; what is already
// queued is Requeue's.
func (p *Player) Duck(on bool) {
	gain := float32(1)
	if on {
		gain = ducked
	}

	p.mu.Lock()
	p.gain = gain
	p.mu.Unlock()
}

// ducking is the level to write at.
func (p *Player) ducking() float32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.gain
}

// Requeue implements speaker.Producer, scaling what is already queued.
func (p *Player) Requeue() {
	p.mu.Lock()
	gain := p.gain
	p.mu.Unlock()

	if gain == 1 {
		return
	}
	speaker.Get().Adjust(func(samples []int16) { speaker.Scale(samples, gain) })
}
