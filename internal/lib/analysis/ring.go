package analysis

import "sync"

type Ring struct {
	mu      sync.Mutex
	buf     []int16
	dropped uint64
}

func NewRing(samples int) *Ring { return &Ring{buf: make([]int16, 0, samples)} }

func (r *Ring) Offer(s []int16) {
	if !r.mu.TryLock() {
		return
	}
	room := cap(r.buf) - len(r.buf)
	if len(s) > room {
		r.dropped += uint64(len(s) - room)
		s = s[:room]
	}
	r.buf = append(r.buf, s...)
	r.mu.Unlock()
}

func (r *Ring) Drain(dst []int16) []int16 {
	r.mu.Lock()
	dst = append(dst[:0], r.buf...)
	r.buf = r.buf[:0]
	r.mu.Unlock()
	return dst
}

func (r *Ring) Dropped() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dropped
}
