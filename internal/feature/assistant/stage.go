package assistant

import (
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	sharedgpu "github.com/ygelfand/libcountertop/pkg/display/gpu"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/feature/voice"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/gpu"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

const (
	AboveUI     = 6
	BelowUI     = 1
	stageEvery  = time.Second / 60
	stageLinger = time.Second
	stageGrace  = 200 * time.Millisecond
	stageHide   = 150 * time.Millisecond
	stageRetire = 3
)

func stageOf(p voice.Phase) (config.Stage, bool) {
	switch p {
	case voice.Listening:
		return config.StageListening, true
	case voice.Thinking:
		return config.StageWaiting, true
	case voice.Replying, voice.Idle:
		return config.StageResponding, true
	}
	return "", false
}

type Look struct {
	Place config.LookPlace
	Stage config.Stage
	Kind  string
}

func (l Look) Custom() bool { return l.Kind != "" }

func LookFor(show Showing) Look {
	st, ok := stageOf(show.Phase)
	if !ok {
		return Look{}
	}
	look := config.Get().Wake.Slot(show.Slot).Look
	return Look{Place: look.Place, Stage: st, Kind: look.Kind(st)}
}

type slot struct {
	layer  *sharedgpu.Layer
	vis    visual.Visual
	kind   string
	base   int
	z      int
	placed ui.Rect
	rot    display.Orientation
	fresh  bool
	last   time.Time
}

func (sl *slot) close() {
	if sl != nil && sl.layer != nil {
		_ = sl.layer.Close()
		sl.layer = nil
	}
}

func (sl *slot) fits(w wanted) bool {
	return sl.layer != nil && sl.layer.W == w.at.W && sl.layer.H == w.at.H && sl.base == w.z &&
		sl.layer.Alive()
}

type stager struct {
	shown atomic.Bool
	full  atomic.Bool

	mu      sync.Mutex
	want    ui.Rect
	z       int
	kind    string
	source  config.Source
	seen    time.Time
	running bool

	cur      *slot
	next     *slot
	retiring *slot
	retireIn int
	broken   string
	release  func()
	hiding   time.Time
}

var stage = &stager{}

func Staged() bool { return stage.shown.Load() }

func StagedFull() bool { return Staged() && stage.full.Load() }

func Stage(at ui.Rect, z int, l Look) {
	if !l.Custom() || at.W <= 0 || at.H <= 0 {
		return
	}
	s := stage
	s.mu.Lock()
	defer s.mu.Unlock()
	s.want, s.z, s.kind, s.source, s.seen = at, z, l.Kind, l.Stage.Source(), time.Now()
	if !s.running {
		s.running = true
		go s.run()
	}
}

func Unstage() {
	s := stage
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		s.seen = time.Time{}
	}
}

func (s *stager) run() {
	t := time.NewTicker(stageEvery)
	defer t.Stop()
	for range t.C {
		s.mu.Lock()
		linger := stageLinger
		if s.z != BelowUI {
			linger = stageGrace
		}
		if time.Since(s.seen) <= linger {
			s.hiding = time.Time{}
		} else if s.z != BelowUI {
			s.close()
			s.running = false
			s.mu.Unlock()
			return
		} else if s.hiding.IsZero() {
			s.hiding = time.Now()
			s.shown.Store(false)
			Get().Hide()
		} else if time.Since(s.hiding) > stageHide {
			s.close()
			s.hiding = time.Time{}
			s.running = false
			s.mu.Unlock()
			return
		}
		w := wanted{at: s.want, z: s.z, kind: s.kind, source: s.source}
		hiding := !s.hiding.IsZero()
		s.mu.Unlock()
		s.frame(w, hiding)
	}
}

type wanted struct {
	at     ui.Rect
	z      int
	kind   string
	source config.Source
}

func (s *stager) frame(w wanted, hiding bool) {
	if s.release == nil {
		s.release = visuals.Get().Hold()
	}
	if s.cur != nil && !s.cur.fits(w) {
		s.drop()
	}
	if s.cur == nil {
		if w.kind == s.broken {
			return
		}
		if s.cur = s.open(w, w.z, w.at); s.cur == nil {
			return
		}
		s.full.Store(w.z == BelowUI)
	}

	in := visuals.From(visuals.Get().Input(), w.source)
	if !s.shade(s.cur, w.at, w.at, in) {
		s.drop()
		return
	}
	if !hiding && !s.shown.Load() {
		s.shown.Store(true)
		Get().Paint()
	}
	if s.retiring != nil {
		if s.retireIn--; s.retireIn <= 0 {
			s.retiring.close()
			s.retiring = nil
		}
	}

	if s.cur.kind == w.kind || w.kind == s.broken {
		s.next.close()
		s.next = nil
		return
	}
	if s.next != nil && (s.next.kind != w.kind || !s.next.fits(w)) {
		s.next.close()
		s.next = nil
	}
	off := w.at
	off.X += off.W * 2
	if s.next == nil {
		z := w.z
		if s.cur.z == w.z {
			z = w.z + 1
		}
		if s.next = s.open(w, z, off); s.next == nil {
			return
		}
	}
	if !s.shade(s.next, off, w.at, in) {
		s.next.close()
		s.next = nil
		return
	}
	if !s.place(s.next, w.at) {
		s.next.close()
		s.next = nil
		return
	}
	s.retiring.close()
	s.retiring, s.retireIn = s.cur, stageRetire
	s.cur, s.next = s.next, nil
}

func (s *stager) open(w wanted, z int, at ui.Rect) *slot {
	l, err := gpu.Get().OpenAt(w.at.W, w.at.H, z)
	if errors.Is(err, sharedgpu.ErrNoHelper) {
		return nil
	}
	if err != nil {
		slog.Error("assistant: opening the GPU layer", "err", err)
		s.broken = w.kind
		return nil
	}
	sl := &slot{
		layer: l,
		vis:   visual.New(visual.Kind(w.kind)),
		kind:  w.kind,
		base:  w.z,
		z:     z,
		fresh: true,
	}
	if !s.place(sl, at) {
		sl.close()
		return nil
	}
	return sl
}

func (s *stager) place(sl *slot, at ui.Rect) bool {
	rot := display.Get().Orientation()
	if sl.placed == at && sl.rot == rot {
		return true
	}
	if err := sl.layer.Place(at); err != nil {
		slog.Error("assistant: placing the GPU layer", "err", err)
		return false
	}
	sl.placed, sl.rot = at, rot
	return true
}

func (s *stager) shade(sl *slot, placed, area ui.Rect, in visual.Input) bool {
	if !s.place(sl, placed) {
		return false
	}
	now := time.Now()
	if !sl.last.IsZero() {
		in.Dt = now.Sub(sl.last)
	}
	sl.last = now
	if err := sl.vis.Shade(sl.layer, sl.fresh, area, in); err != nil {
		slog.Error(
			"assistant: GPU visual failed",
			"kind",
			sl.kind,
			"alive",
			sl.layer.Alive(),
			"fresh",
			sl.fresh,
			"err",
			err,
		)
		if sl.layer.Alive() {
			s.broken = sl.kind
		}
		return false
	}
	sl.fresh = false
	return true
}

func (s *stager) drop() {
	s.shown.Store(false)
	s.cur.close()
	s.next.close()
	s.retiring.close()
	s.cur, s.next, s.retiring = nil, nil, nil
}

func (s *stager) close() {
	s.drop()
	if s.release != nil {
		s.release()
		s.release = nil
	}
}
