package touch

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/ygelfand/libcountertop/pkg/hook"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/service"
)

func init() {
	component.Register(component.Hardware, Get, component.Order(30),
		component.Supervise(service.Restart(time.Second, time.Minute)))
}

type Phase int

const (
	Down Phase = iota
	Move
	Up
)

func (p Phase) String() string {
	switch p {
	case Down:
		return "down"
	case Move:
		return "move"
	case Up:
		return "up"
	}
	return "unknown"
}

type Contact struct {
	Slot  int
	ID    int
	X, Y  int
	Phase Phase

	At time.Time
}

func (c Contact) String() string {
	return fmt.Sprintf("%s at %d,%d (slot %d, id %d)", c.Phase, c.X, c.Y, c.Slot, c.ID)
}

type Screen struct {
	Contacts hook.Hook[Contact]

	Gestures hook.Hook[Gesture]

	mu  sync.Mutex
	f   *os.File
	err error
	rec *Recognizer
	rot display.Orientation

	last time.Time
}

var (
	once   sync.Once
	shared *Screen
)

func Get() *Screen { once.Do(func() { shared = &Screen{} }); return shared }

func (s *Screen) Name() string { return "touch" }

func (s *Screen) Since() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.last.IsZero() {
		return time.Since(started)
	}
	return time.Since(s.last)
}

var started = time.Now()

func (s *Screen) Startup() component.Progress {
	s.mu.Lock()
	open, err := s.f != nil, s.err
	s.mu.Unlock()

	switch {
	case open:
		return component.Progress{Done: true}
	case err != nil:
		return component.Progress{Failed: true, Doing: err.Error()}
	}
	return component.Progress{Doing: "taking the touchscreen"}
}

func (s *Screen) Start(context.Context) error {
	f, err := open()

	s.mu.Lock()
	s.f, s.err = f, err
	s.mu.Unlock()

	return err
}

func open() (*os.File, error) {
	path, err := Find(board.Current().Touch)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("touch: %s: %w", path, err)
	}
	return f, nil
}

func (s *Screen) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.f == nil {
		return nil
	}
	err := s.f.Close()
	s.f = nil
	return err
}

func (s *Screen) Run(ctx context.Context) error {
	s.mu.Lock()
	f := s.f
	s.mu.Unlock()

	if f == nil {
		return fmt.Errorf("touch: not open")
	}

	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()

	var d decoder
	return read(f, func(e rawEvent) {
		for _, c := range d.event(e) {
			s.Deliver(c)
		}
	})
}

func (s *Screen) Deliver(c Contact) {
	s.mu.Lock()
	s.last = time.Now()
	s.mu.Unlock()

	s.Contacts.Emit(c)

	if g, ok := s.recognize(c); ok {
		slog.Debug("gesture", "kind", g.Kind, "from", g.From, "toward", g.Toward,
			"at", fmt.Sprintf("%d,%d", g.EndX, g.EndY))
		s.Gestures.Emit(g)
	}
}

func (s *Screen) recognize(c Contact) (Gesture, bool) {
	rot := display.Get().Orientation()

	s.mu.Lock()
	if s.rec == nil || s.rot != rot {
		w, h := rot.Size(board.Current().PanelWidth, board.Current().PanelHeight)
		s.rec, s.rot = NewRecognizer(w, h), rot
	}
	rec := s.rec
	s.mu.Unlock()

	return rec.Feed(c)
}

func rotate(px, py int) (x, y int) {
	return display.Get().
		Orientation().
		Unproject(board.Current().PanelWidth, board.Current().PanelHeight, px, py)
}
