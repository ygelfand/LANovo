// Package touch reads the touchscreen.
//
// A Goodix gt9xx on i2c-3, multi-touch protocol B: contacts live in numbered slots and keep a
// tracking id until they are lifted. The panel reports its native portrait, so positions are
// rotated into the coordinates the display draws in.
package touch

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/service"
	"github.com/ygelfand/libcountertop/pkg/hook"
)

func init() {
	component.Register(component.Hardware, Get, component.Order(30),
		component.Supervise(service.Restart(time.Second, time.Minute)))
}

// Phase is what happened to a contact.
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

// Contact is one finger, in the coordinates the display draws in.
type Contact struct {
	Slot  int
	ID    int
	X, Y  int
	Phase Phase

	// At is when the report arrived, which is what a gesture measures against.
	At time.Time
}

func (c Contact) String() string {
	return fmt.Sprintf("%s at %d,%d (slot %d, id %d)", c.Phase, c.X, c.Y, c.Slot, c.ID)
}

// Screen is the touchscreen.
type Screen struct {
	// Contacts carries every change to a finger on the glass.
	Contacts hook.Hook[Contact]

	// Gestures carries what each contact turned out to be, once it has lifted.
	Gestures hook.Hook[Gesture]

	mu  sync.Mutex
	f   *os.File
	err error
	rec *Recognizer
	rot display.Orientation

	// last is when the glass was last touched, for anything that wants to know whether somebody is
	// there. Zero until the first touch, which reads as having been a very long time.
	last time.Time
}

var (
	once   sync.Once
	shared *Screen
)

// Get is the touchscreen. Nothing here touches hardware: Start does.
func Get() *Screen { once.Do(func() { shared = &Screen{} }); return shared }

func (s *Screen) Name() string { return "touch" }

// Since is how long it has been since anybody touched the glass, for anything that wants to know
// whether somebody is in front of the device.
//
// A fact about the hardware rather than a policy: what counts as long enough to be left alone is
// the caller's business, and different callers will answer it differently. A device nobody has
// touched since it started reads as having been alone since it started, which is true.
func (s *Screen) Since() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.last.IsZero() {
		return time.Since(started)
	}
	return time.Since(s.last)
}

// started is when the process came up, which is the best answer available before the first touch.
var started = time.Now()

// Startup is ready once the input device is open. A device with no glass is still a device worth
// having, so failing to take it is a fault the screen shows rather than one it waits on.
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

// Start opens the input device.
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

// Close releases the device.
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

// Run reads until ctx is canceled.
func (s *Screen) Run(ctx context.Context) error {
	s.mu.Lock()
	f := s.f
	s.mu.Unlock()

	if f == nil {
		return fmt.Errorf("touch: not open")
	}

	// A read blocks until a finger moves, which may be never, so canceling closes the device out
	// from under it rather than waiting.
	go func() {
		<-ctx.Done()
		s.Close()
	}()

	var d decoder
	return read(f, func(e rawEvent) {
		for _, c := range d.event(e) {
			s.Deliver(c)
		}
	})
}

// Deliver passes a contact on as though a finger made it, which is what lets the screen be driven
// without one.
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

// recognize follows a contact through to a gesture, resizing when the device has been turned:
// the recognizer works in the picture, and the picture changes shape.
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

// rotate turns a panel position into the coordinates the display draws in, through the same
// orientation the panel projects with. Asked for each time: a device that has been turned reports
// touches at the rotation it is now showing.
func rotate(px, py int) (x, y int) {
	return display.Get().
		Orientation().
		Unproject(board.Current().PanelWidth, board.Current().PanelHeight, px, py)
}
