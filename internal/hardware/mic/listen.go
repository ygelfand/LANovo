package mic

import (
	"log/slog"
	"sync"

	"github.com/ygelfand/libcountertop/pkg/hook"
)

const tapDepth = 8

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

	if t.dropped > 0 {
		slog.Warn("listener behind", "who", t.name, "frames", t.dropped)
	}
}

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
