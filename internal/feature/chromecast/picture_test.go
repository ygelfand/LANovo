package chromecast

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/video"
)

type pictured struct {
	ending
	webm  []byte
	reads int
}

func (p *pictured) Read([]int16) (int, error) {
	p.mu.Lock()
	p.reads++
	p.mu.Unlock()
	return 0, nil
}

func (p *pictured) readCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reads
}

func TestTheSoundWaitsForThePicture(t *testing.T) {
	o := newOutput()
	o.q = &fakeQueue{}
	start := make(chan func(), 1)
	o.play = func(ctx context.Context, s video.Stream) (video.Report, error) {
		start <- s.Started
		<-ctx.Done()
		return video.Report{}, nil
	}
	src := &pictured{webm: vp9Head()}
	o.Play(src)
	t.Cleanup(func() { o.Stop(src) })

	started := <-start
	time.Sleep(150 * time.Millisecond)
	if n := src.readCount(); n != 0 {
		t.Fatalf("the sound was read %d times before any picture", n)
	}
	started()
	deadline := time.Now().Add(2 * time.Second)
	for src.readCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if src.readCount() == 0 {
		t.Error("the sound never started after the first picture")
	}
}

func (p *pictured) Pictured() bool { return p.webm != nil }

func (p *pictured) Pictures(context.Context) (io.ReadCloser, time.Duration, error) {
	if p.webm == nil {
		return nil, 0, nil
	}
	return io.NopCloser(bytes.NewReader(p.webm)), 0, nil
}

func el(id []byte, body ...[]byte) []byte {
	b := bytes.Join(body, nil)
	return append(append(append([]byte(nil), id...), 0x80|byte(len(b))), b...)
}

func vp9Head() []byte {
	return el([]byte{0x18, 0x53, 0x80, 0x67}, el([]byte{0x16, 0x54, 0xAE, 0x6B}, el([]byte{0xAE},
		el([]byte{0xD7}, []byte{1}),
		el([]byte{0x86}, []byte("V_VP9")),
		el([]byte{0xE0}, el([]byte{0xB0}, []byte{0x07, 0x80}), el([]byte{0xBA}, []byte{0x04, 0x38})),
	)))
}

func TestAPicturedSourcePlaysAgainstWhatHasBeenHeard(t *testing.T) {
	q := &fakeQueue{}
	o := newOutput()
	o.q = q
	got := make(chan video.Stream, 1)
	o.play = func(_ context.Context, s video.Stream) (video.Report, error) {
		got <- s
		return video.Report{}, nil
	}
	src := &pictured{ending: ending{start: 10 * time.Second}, webm: vp9Head()}
	o.Play(src)
	t.Cleanup(func() { o.Stop(src) })

	var s video.Stream
	select {
	case s = <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("the pictures never played")
	}
	if s.Codec != video.VP9 || s.Width != 1920 || s.Height != 1080 {
		t.Errorf("stream %+v", s)
	}
	if at, running := s.Clock(); at != 10*time.Second || !running {
		t.Errorf("clock %v running %v, want 10s: where it starts, nothing heard yet", at, running)
	}
	o.Pause(src)
	if _, running := s.Clock(); running {
		t.Error("the clock runs while paused")
	}
	o.Resume(src)
	s.Waiting(true)
	if _, running := s.Clock(); running {
		t.Error("the clock runs while the picture is waiting for frames")
	}
	s.Waiting(false)
	if _, running := s.Clock(); !running {
		t.Error("the clock stays stopped once the picture has frames again")
	}
}

func TestAPictureThatFailsStopsTheSoundToo(t *testing.T) {
	o := newOutput()
	o.q = &fakeQueue{}
	o.play = func(context.Context, video.Stream) (video.Report, error) {
		return video.Report{}, errors.New("refused")
	}
	src := &pictured{webm: vp9Head()}
	o.Play(src)
	t.Cleanup(func() { o.Stop(src) })

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		o.mu.Lock()
		cur := o.cur
		o.mu.Unlock()
		if cur == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("the sound is still playing after its picture failed")
}

func TestWaitingForFramesHoldsTheSound(t *testing.T) {
	q := &fakeQueue{}
	o := newOutput()
	o.q = q
	got := make(chan video.Stream, 1)
	o.play = func(ctx context.Context, s video.Stream) (video.Report, error) {
		s.Started()
		got <- s
		<-ctx.Done()
		return video.Report{}, nil
	}
	src := &loud{}
	o.Play(src)
	t.Cleanup(func() { o.Stop(src) })
	s := <-got
	filled(t, q)

	s.Waiting(true)
	time.Sleep(200 * time.Millisecond)
	if n := q.Queued(); n != 0 {
		t.Errorf("%d frames queued for the speaker while the picture waits", n)
	}
	s.Waiting(false)
	if q.Queued() == 0 {
		t.Error("the sound did not come back with the picture")
	}
}

type loud struct{ pictured }

func (l *loud) Read(pcm []int16) (int, error) { return len(pcm), nil }
func (l *loud) Pictured() bool                { return true }
func (l *loud) Pictures(context.Context) (io.ReadCloser, time.Duration, error) {
	return io.NopCloser(bytes.NewReader(vp9Head())), 0, nil
}

func TestASourceWithoutPicturesPlaysNone(t *testing.T) {
	o := newOutput()
	o.q = &fakeQueue{}
	played := make(chan struct{}, 1)
	o.play = func(context.Context, video.Stream) (video.Report, error) {
		played <- struct{}{}
		return video.Report{}, nil
	}
	src := &pictured{}
	o.Play(src)
	t.Cleanup(func() { o.Stop(src) })
	select {
	case <-played:
		t.Error("a source with no picture stream played one")
	case <-time.After(200 * time.Millisecond):
	}
}
