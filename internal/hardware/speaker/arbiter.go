package speaker

import (
	"fmt"
	"log/slog"
	"sync"
)

// Producer is a background sound: a track, or a noise machine playing along with the room.
type Producer interface {
	// Stand says whether this producer may be heard. It is the standing itself rather than a change
	// to it: the arbiter delivers it again whenever anything moves, and the same value twice does
	// nothing.
	Stand(down bool)

	// Duck sets the level for what is written next; what is already queued is Requeue's job.
	Duck(on bool)

	// Requeue re-scales what is already queued, which can be seconds of audio.
	Requeue()
}

// Arbiter is the one background the driver holds, standing in for however many there are. The
// newest plays and the rest wait in order behind it, so when a track ends the stream it interrupted
// carries on.
//
// Who may be heard is worked out from the stack and the hold each time either moves, and delivered
// whole. Nothing here counts what it has handed out: a producer that would have been left owing a
// resume is instead told its standing again by the next thing to happen.
type Arbiter struct {
	mu    sync.Mutex
	stack []Producer // the last is the one being heard
	held  bool       // something has stood the background down
	duck  bool
}

// Took is a producer starting. Whatever was playing stands down but keeps its place.
func (a *Arbiter) Took(p Producer) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.drop(p)
	if stood := a.top(); stood != nil && stood != p {
		slog.Debug("background handover", "from", kind(stood), "to", kind(p))
	}
	a.stack = append(a.stack, p)

	p.Duck(a.duck)
	a.tell()
}

// Gave is a producer finishing. Whatever it interrupted picks up again.
func (a *Arbiter) Gave(p Producer) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.drop(p)
	if now := a.top(); now != nil && !a.held {
		slog.Debug("background resumed", "who", kind(now), "after", kind(p))
	}

	// One that has left is contending for nothing, so it is standing down for nothing either. Saying
	// so here is what keeps a stop during a hold from leaving it held for the next track.
	p.Stand(false)
	a.tell()
}

// Stand is what the driver holds: something wants the speaker to itself, so whichever producer is
// being heard stands down for it, and the rest stay down until it is over.
func (a *Arbiter) Stand(down bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.held = down
	a.tell()
}

// Owns reports whether the queue is this producer's to empty. It is not, while something holds the
// speaker or another producer is the one being heard: the audio in it belongs to them.
func (a *Arbiter) Owns(p Producer) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.held {
		return false
	}
	top := a.top()
	return top == nil || top == p
}

// Duck quietens everything, waiting producers included, so one resuming mid-turn comes back quiet.
func (a *Arbiter) Duck(on bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.duck == on {
		return
	}
	a.duck = on

	// Delivered under the lock, like the standing: a turn ending cannot overtake a turn beginning
	// and leave the level on with nothing left to take it off.
	for _, p := range a.stack {
		p.Duck(on)
	}

	// Only the audible one, or the same samples get attenuated once per producer. Unducking is left
	// to drain.
	if heard := a.top(); on && heard != nil {
		heard.Requeue()
	}
}

// Playing is the producer being heard, or nil. For diagnostics.
func (a *Arbiter) Playing() Producer {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.top()
}

// tell passes on where everything in the stack currently stands. Called with the lock held, so a
// decision and its delivery cannot be overtaken by each other: a producer never calls back into the
// arbiter from inside one, and the ones that do call in release their own locks first.
func (a *Arbiter) tell() {
	top := a.top()
	for _, p := range a.stack {
		p.Stand(a.held || p != top)
	}
}

// top and drop are called with the lock held.
func (a *Arbiter) top() Producer {
	if len(a.stack) == 0 {
		return nil
	}
	return a.stack[len(a.stack)-1]
}

func (a *Arbiter) drop(p Producer) {
	for i, have := range a.stack {
		if have == p {
			a.stack = append(a.stack[:i], a.stack[i+1:]...)
			return
		}
	}
}

func kind(p Producer) string {
	if p == nil {
		return "nothing"
	}
	return fmt.Sprintf("%T", p)
}
