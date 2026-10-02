package speaker

import (
	"sync"
	"time"
)

const echoFrames = Rate

type echo struct {
	mu       sync.Mutex
	ring     [echoFrames]int16
	next     uint64
	playing  uint64
	at       time.Time
	anchored bool
}

func (e *echo) commit(from uint64, mono []int16, playing uint64, at time.Time, anchored bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, v := range mono {
		e.ring[(from+uint64(i))%echoFrames] = v
	}
	e.next = from + uint64(len(mono))
	if anchored {
		e.playing, e.at, e.anchored = playing, at, true
	}
}

func (e *echo) read(from uint64, dst []int16) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	end := from + uint64(len(dst))
	if end > e.next || end < from || e.next-from > echoFrames {
		return false
	}
	for i := range dst {
		dst[i] = e.ring[(from+uint64(i))%echoFrames]
	}
	return true
}

func (e *echo) anchor() (uint64, time.Time, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.playing, e.at, e.anchored
}

func (s *Speaker) Echo(from uint64, dst []int16) bool { return s.echo.read(from, dst) }

func (s *Speaker) Playing() (frame uint64, at time.Time, ok bool) { return s.echo.anchor() }
