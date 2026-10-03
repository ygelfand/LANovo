package video

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/lib/surface"
)

type Frame struct {
	Data  []byte
	At    time.Duration
	Crypt *surface.Crypt
}

type Source interface {
	Next() (Frame, error)
}

type Clock func() (at time.Duration, running bool)

type Stream struct {
	Codec         uint32
	Width, Height uint32
	Session       uint32
	Source        Source
	Clock         Clock
	Over          Overlay
	Started       func()
	Waiting       func(bool)
}

type Band struct {
	Pix    []byte
	Stride int
	At     display.Rect
}

type Overlay interface {
	Band(o display.Orientation, fw, fh int) (Band, bool)
	Changed() <-chan struct{}
}

type Report struct {
	Shown, Dropped int
	Turns          int
	Orientation    display.Orientation
	At             display.Rect
}

const (
	videoZ    = -2
	controlsZ = -1
	buffered  = 90
	clockTick = 50 * time.Millisecond
	full      = 5 * time.Millisecond
)

var ids atomic.Uint32

func fit(fw, fh, w, h int) display.Rect {
	dw, dh := fw, fw*h/w
	if dh > fh {
		dw, dh = fh*w/h, fh
	}
	return display.Rect{X: (fw - dw) / 2, Y: (fh - dh) / 2, W: dw, H: dh}
}

func placement(o display.Orientation, fw, fh, w, h int) (x, y float32, m surface.Matrix, at display.Rect) {
	vw, vh := o.Size(fw, fh)
	return o.Place(fw, fh, fit(vw, vh, w, h), w, h)
}

func codecOf(fourcc uint32) (uint32, error) {
	switch fourcc {
	case VP9:
		return surface.VP9, nil
	case H264:
		return surface.AVC, nil
	}
	return 0, fmt.Errorf("video: no decoder for %08x", fourcc)
}

type staller interface {
	Stalling()
}

type resumer interface {
	Resuming()
}

type keeper interface {
	Reuse(c *surface.Client, s spec) (uint32, bool)
	Adopt(c *surface.Client, id uint32, s spec)
	Supersede(id uint32)
}

const stallAfter = 400 * time.Millisecond

type stallWatch struct {
	shown, moved time.Time
	at           time.Duration
}

func (w *stallWatch) show(t time.Time) { w.shown = t }

func (w *stallWatch) stalled(t time.Time, now time.Duration, running bool, shown, pending int, drained bool) bool {
	if now != w.at {
		w.at, w.moved = now, t
	}
	if !running {
		w.shown, w.moved = t, t
		return false
	}
	if shown == 0 || drained || t.Sub(w.shown) < stallAfter {
		return false
	}
	return pending == 0 || t.Sub(w.moved) >= stallAfter
}

type Screen interface {
	Orientation() display.Orientation
	Native() (w, h int)
}

func Play(ctx context.Context, s Stream) (Report, error) {
	return On(ctx, &Beneath{}, s)
}

func On(ctx context.Context, p Screen, s Stream) (Report, error) {
	c := display.Get().Helper()
	if c == nil {
		return Report{}, errors.New("video: the display helper is not connected")
	}
	codec, err := codecOf(s.Codec)
	if err != nil {
		return Report{}, err
	}
	want := spec{codec: codec, w: int(s.Width), h: int(s.Height), session: s.Session}
	k, keeps := p.(keeper)
	var id uint32
	reused := false
	if keeps {
		if kid, ok := k.Reuse(c, want); ok {
			if err := c.VideoFlush(kid); err == nil {
				id, reused = kid, true
			}
		}
	}
	if !reused {
		id = 1000 + ids.Add(1)
		decoder := ""
		if s.Session != 0 {
			decoder = board.Current().SecureDecoders[surface.MIME(codec)]
		}
		if err := c.VideoOpen(id, codec, want.w, want.h, videoZ, s.Session, decoder); err != nil {
			return Report{}, fmt.Errorf("video: opening the decoder: %w", err)
		}
	}
	if at, running := s.Clock(); c.VideoClock(id, at, running) != nil {
		return Report{}, errors.New("video: the display helper went away")
	}
	kept := reused
	defer func() {
		if keeps && kept {
			k.Adopt(c, id, want)
			return
		}
		c.VideoClose(id)
	}()

	fw, fh := p.Native()
	var rep Report
	visible := reused
	face := func(o display.Orientation) error {
		x, y, m, at := placement(o, fw, fh, int(s.Width), int(s.Height))
		rep.Orientation, rep.At = o, at
		return c.VideoPlace(id, x, y, m, videoZ, visible)
	}
	if err := face(p.Orientation()); err != nil {
		return rep, fmt.Errorf("video: placing the picture: %w", err)
	}

	ctrl := newControls(c, s.Over, fw, fh)
	defer ctrl.close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		t := time.NewTicker(clockTick)
		defer t.Stop()
		for {
			at, running := s.Clock()
			if c.VideoClock(id, at, running) != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()

	frames := make(chan Frame, buffered)
	failed := make(chan error, 1)
	go func() {
		defer close(frames)
		for {
			f, err := s.Source.Next()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					failed <- err
				}
				return
			}
			select {
			case frames <- f:
			case <-ctx.Done():
				return
			}
		}
	}()

	var changed <-chan struct{}
	if s.Over != nil {
		changed = s.Over.Changed()
	}
	look := time.NewTicker(250 * time.Millisecond)
	defer look.Stop()

	var watch stallWatch
	var last time.Duration
	started := false
	drained, waiting, stalled := false, false, false
	defer func() {
		if waiting && s.Waiting != nil {
			s.Waiting(false)
		}
	}()
	played := func(pl surface.Played) {
		if int(pl.Shown) > rep.Shown {
			watch.show(time.Now())
			kept = true
			if stalled {
				stalled = false
				if r, ok := p.(resumer); ok {
					r.Resuming()
				}
			}
		}
		rep.Shown, rep.Dropped = int(pl.Shown), int(pl.Dropped)
	}
	feed := func(f Frame, flags uint32) (surface.Played, error) {
		for {
			var pl surface.Played
			var err error
			if f.Crypt != nil {
				pl, err = c.VideoCrypt(id, f.At, flags, f.Crypt, f.Data)
			} else {
				pl, err = c.VideoSample(id, f.At, flags, f.Data)
			}
			if !errors.Is(err, surface.Full) {
				return pl, err
			}
			played(pl)
			select {
			case <-ctx.Done():
				return pl, ctx.Err()
			case <-time.After(full):
			}
		}
	}

	for {
		var next <-chan Frame
		if !drained {
			next = frames
		}
		select {
		case <-ctx.Done():
			return rep, ctx.Err()
		case err := <-failed:
			return rep, err
		case <-changed:
			ctrl.update(p.Orientation())
			continue
		case <-look.C:
		case f, ok := <-next:
			if !ok {
				drained = true
				if _, err := feed(Frame{At: last}, surface.SampleEnd); err != nil {
					return rep, fmt.Errorf("video: ending the stream: %w", err)
				}
				continue
			}
			last = f.At
			pl, err := feed(f, 0)
			if err != nil {
				return rep, fmt.Errorf("video: decoding: %w", err)
			}
			played(pl)
		}

		if o := p.Orientation(); o != rep.Orientation {
			if err := face(o); err != nil {
				return rep, err
			}
			ctrl.update(o)
			rep.Turns++
		}

		now, running := s.Clock()
		if !started && rep.Shown > 0 {
			started = true
			if !visible {
				visible = true
				if err := face(rep.Orientation); err != nil {
					return rep, err
				}
			}
			if keeps {
				k.Supersede(id)
			}
			if s.Started != nil {
				s.Started()
			}
		}
		if watch.stalled(time.Now(), now, running, rep.Shown, len(frames), drained) {
			if st, ok := p.(staller); ok && !stalled {
				st.Stalling()
			}
			stalled = true
			if len(frames) == 0 && !waiting {
				waiting = true
				if s.Waiting != nil {
					s.Waiting(true)
				}
			}
		}
		if waiting && (len(frames) > 0 || drained) {
			waiting = false
			if s.Waiting != nil {
				s.Waiting(false)
			}
		}
		if drained && (now >= last || !running) {
			return rep, nil
		}
	}
}

type controls struct {
	c      *surface.Client
	over   Overlay
	fw, fh int
	layer  *surface.Layer
	shown  display.Rect
}

func newControls(c *surface.Client, over Overlay, fw, fh int) *controls {
	k := &controls{c: c, over: over, fw: fw, fh: fh}
	if over == nil {
		return k
	}
	id := 1000 + ids.Add(1)
	l, err := c.Create(id, 0, 0, fw, fh, controlsZ, 0)
	if err != nil {
		return k
	}
	k.layer = l
	return k
}

func (k *controls) update(o display.Orientation) {
	if k.layer == nil {
		return
	}
	b, ok := k.over.Band(o, k.fw, k.fh)
	dirty := k.shown
	clearRect(k.layer, k.shown)
	k.shown = display.Rect{}
	if ok && b.At.W > 0 && b.At.H > 0 && b.At.X >= 0 && b.At.Y >= 0 && b.At.X+b.At.W <= k.fw && b.At.Y+b.At.H <= k.fh {
		for y := 0; y < b.At.H; y++ {
			dst := k.layer.Pixels[(b.At.Y+y)*k.layer.Stride+b.At.X*4:]
			copy(dst[:b.At.W*4], b.Pix[y*b.Stride:y*b.Stride+b.At.W*4])
		}
		k.shown = b.At
		dirty = union(dirty, b.At)
	}
	if dirty.W > 0 && dirty.H > 0 {
		k.c.Frame(k.layer.ID, 0, []surface.Rect{{X: dirty.X, Y: dirty.Y, W: dirty.W, H: dirty.H}})
	}
}

func (k *controls) close() {
	if k.layer != nil {
		k.c.Destroy(k.layer.ID)
	}
}

func clearRect(l *surface.Layer, r display.Rect) {
	for y := r.Y; y < r.Y+r.H; y++ {
		clear(l.Pixels[y*l.Stride+r.X*4 : y*l.Stride+(r.X+r.W)*4])
	}
}

func union(a, b display.Rect) display.Rect {
	if a.W <= 0 || a.H <= 0 {
		return b
	}
	if b.W <= 0 || b.H <= 0 {
		return a
	}
	x0, y0 := min(a.X, b.X), min(a.Y, b.Y)
	x1, y1 := max(a.X+a.W, b.X+b.W), max(a.Y+a.H, b.Y+b.H)
	return display.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}
