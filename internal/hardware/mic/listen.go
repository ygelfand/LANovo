package mic

import (
	"log/slog"
	"sync"

	"github.com/ygelfand/LANovo/internal/lib/hook"
)

// tapDepth is how many frames a listener may fall behind by. Short on purpose: audio nobody took in
// time is a moment not heard, and queueing it only moves the problem later in the conversation.
const tapDepth = 8

// tap is one listener's channel and what it has missed.
type tap struct {
	name string

	mu      sync.Mutex
	ch      chan []int16
	closed  bool
	dropped int
}

func (t *tap) send(frame []int16) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return
	}
	select {
	case t.ch <- frame:
	default:
		t.dropped++
	}
}

func (t *tap) close() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return
	}
	t.closed = true
	close(t.ch)

	// A turn lasts seconds, so anything it lost would otherwise go unsaid.
	if t.dropped > 0 {
		slog.Warn("listener behind", "who", t.name, "frames", t.dropped)
	}
}

// Listen hands out voice-rate frames on a channel of the caller's own, and the function that stops
// it and closes the channel.
//
// The hook runs its listeners on the capture reader, which must not wait: this is where that reader
// stops being the caller's problem. Sends never block, and the channel is only closed from here, so
// a frame arriving as the listener goes away is dropped rather than sent to a closed channel.
func (m *Mics) Listen(name string) (<-chan []int16, func()) {
	return listen(name, &m.Speech)
}

func (m *Mics) ListenStereo(name string) (<-chan []int16, func()) {
	return listen(name, &m.Heard)
}

func listen(name string, from *hook.Hook[Frame]) (<-chan []int16, func()) {
	t := &tap{name: name, ch: make(chan []int16, tapDepth)}

	cancel := from.Listen(func(f Frame) { t.send(f.Samples) })

	var once sync.Once
	return t.ch, func() {
		once.Do(func() {
			cancel()
			t.close()
		})
	}
}
