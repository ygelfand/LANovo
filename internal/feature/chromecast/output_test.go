package chromecast

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/libcountertop/pkg/media/playback"
)

type ending struct {
	mu    sync.Mutex
	read  bool
	start time.Duration
}

func (e *ending) Read(pcm []int16) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.read {
		return 0, io.EOF
	}
	e.read = true
	return len(pcm), nil
}

func (e *ending) Showing() playback.Showing { return playback.Showing{} }
func (e *ending) Start() time.Duration      { return e.start }
func (e *ending) Label() string             { return "" }
func (e *ending) Play()                     {}
func (e *ending) Pause()                    {}
func (e *ending) Stop()                     {}
func (e *ending) Next()                     {}
func (e *ending) Previous()                 {}

type fakeQueue struct {
	mu      sync.Mutex
	samples []int16
}

func (h *fakeQueue) Play(s []int16)       { h.mu.Lock(); h.samples = append(h.samples, s...); h.mu.Unlock() }
func (h *fakeQueue) Drain()               { h.Take() }
func (h *fakeQueue) Adjust(func([]int16)) {}

func (h *fakeQueue) Take() []int16 {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.samples
	h.samples = nil
	return s
}

func (h *fakeQueue) Queued() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.samples) / speaker.Channels
}

func TestPausingAfterTheLastReadKeepsTheTail(t *testing.T) {
	q := &fakeQueue{}
	o := newOutput()
	o.q = q
	src := &ending{}
	o.Play(src)
	t.Cleanup(func() { o.Stop(src) })

	until := time.Now().Add(2 * time.Second)
	for q.Queued() == 0 && time.Now().Before(until) {
		time.Sleep(5 * time.Millisecond)
	}
	if q.Queued() == 0 {
		t.Fatal("nothing was queued")
	}

	o.Pause(src)
	time.Sleep(300 * time.Millisecond)

	o.mu.Lock()
	cur := o.cur
	o.mu.Unlock()
	if cur == nil {
		t.Fatal("a pause after the last read ended the track")
	}
	if o.Behind(src) == 0 {
		t.Error("the paused tail was thrown away")
	}
}

type endless struct{ ending }

func (e *endless) Read(pcm []int16) (int, error) { return len(pcm), nil }

func filled(t *testing.T, q *fakeQueue) {
	t.Helper()
	until := time.Now().Add(2 * time.Second)
	for q.Queued() < speaker.Rate && time.Now().Before(until) {
		time.Sleep(5 * time.Millisecond)
	}
	if q.Queued() < speaker.Rate {
		t.Fatal("nothing was queued")
	}
}

func TestPausingStopsTheSoundAndResumingPicksItUp(t *testing.T) {
	q := &fakeQueue{}
	o := newOutput()
	o.q = q
	src := &endless{}
	o.Play(src)
	t.Cleanup(func() { o.Stop(src) })
	filled(t, q)

	o.Pause(src)
	at, paused := o.Position(src)
	time.Sleep(300 * time.Millisecond)
	if n := q.Queued(); n != 0 {
		t.Errorf("%d frames queued for the speaker while paused", n)
	}
	if later, _ := o.Position(src); !paused || later != at {
		t.Errorf("paused %v, position moved from %v to %v", paused, at, later)
	}

	o.Resume(src)
	if _, paused := o.Position(src); paused {
		t.Error("still paused after resuming")
	}
	if q.Queued() == 0 {
		t.Error("what was set aside did not come back")
	}
}

type stuck struct {
	ending
	in      chan struct{}
	release chan struct{}
}

func (s *stuck) Read(pcm []int16) (int, error) {
	select {
	case s.in <- struct{}{}:
	default:
	}
	<-s.release
	return len(pcm), nil
}

func TestAPauseDoesNotWaitForAReadInProgress(t *testing.T) {
	q := &fakeQueue{}
	o := newOutput()
	o.q = q
	src := &stuck{in: make(chan struct{}, 1), release: make(chan struct{})}
	o.Play(src)
	t.Cleanup(func() {
		close(src.release)
		o.Stop(src)
	})
	<-src.in

	done := make(chan struct{})
	go func() {
		o.Pause(src)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pausing waited on a read that had not come back")
	}
	src.release <- struct{}{}
	time.Sleep(100 * time.Millisecond)
	if n := q.Queued(); n != 0 {
		t.Errorf("what the stuck read brought back went to the speaker while paused: %d frames", n)
	}
}

type broken struct {
	stuck
	stops atomic.Int32
}

func (b *broken) Read(pcm []int16) (int, error) {
	select {
	case b.in <- struct{}{}:
	default:
	}
	<-b.release
	return 0, io.ErrUnexpectedEOF
}

func (b *broken) Stop() { b.stops.Add(1) }

func TestAReadThatFailsAfterAHandoffStopsNothing(t *testing.T) {
	o := newOutput()
	o.q = &fakeQueue{}
	src := &broken{stuck: stuck{in: make(chan struct{}, 1), release: make(chan struct{})}}
	o.Play(src)
	<-src.in

	o.Handoff(src)
	close(src.release)
	time.Sleep(200 * time.Millisecond)
	if n := src.stops.Load(); n != 0 {
		t.Errorf("a read cut short by the handoff stopped the source %d times", n)
	}
}

func TestAMetadataChangeLeavesAPauseAlone(t *testing.T) {
	q := &fakeQueue{}
	o := newOutput()
	o.q = q
	src := &endless{}
	o.Play(src)
	t.Cleanup(func() { o.Stop(src) })
	filled(t, q)

	o.Pause(src)
	o.Changed(src)
	time.Sleep(200 * time.Millisecond)
	if _, paused := o.Position(src); !paused || q.Queued() != 0 {
		t.Errorf("paused %v with %d frames queued after a metadata change", paused, q.Queued())
	}
}

func TestPositionIsWhatHasBeenHeard(t *testing.T) {
	q := &fakeQueue{}
	o := newOutput()
	o.q = q
	src := &endless{ending{start: 30 * time.Second}}
	o.Play(src)
	t.Cleanup(func() { o.Stop(src) })
	filled(t, q)

	o.feed.Lock()
	q.mu.Lock()
	q.samples = q.samples[speaker.Rate/2*speaker.Channels:]
	q.mu.Unlock()
	o.feed.Unlock()
	o.Pause(src)

	if at, _ := o.Position(src); at != 30*time.Second+500*time.Millisecond {
		t.Errorf("position %v after half a second played from 30s", at)
	}
}

func TestAnAttachedSenderKeepsTheCardAfterAStop(t *testing.T) {
	o := newOutput()
	o.q = &fakeQueue{}
	t.Cleanup(func() { media.Get().External(nil) })

	o.Attend(&playback.Session{Label: "Pixel"})
	if s, _ := media.Get().Sourced(); s != "*chromecast.standby" {
		t.Fatalf("attached, the card is %s", s)
	}
	if !media.Get().Now().Hold {
		t.Error("the standby card does not hold the player")
	}

	src := &ending{}
	o.Play(src)
	o.Failed(errors.New("refused"))
	o.Stop(src)
	if s, _ := media.Get().Sourced(); s != "*chromecast.standby" {
		t.Errorf("after a stop, the card is %s", s)
	}
	if now := media.Get().Now(); now.Title != "refused" {
		t.Errorf("the card says %q", now.Title)
	}

	o.Attend(nil)
	if _, held := media.Get().Sourced(); held {
		t.Error("the last sender left and the card stayed")
	}
}
