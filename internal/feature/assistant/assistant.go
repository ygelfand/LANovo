// Package assistant is the voice turn on the panel.
//
// echolocal says what a turn is doing on its LED ring. This device has a screen, so the conversation
// says which phase it is in and what words it has, and this draws them. Nothing here decides
// anything about the turn: it shows one, and passes a touch back as the one gesture that ends it.
package assistant

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/voice"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/hardware/touch"
	"github.com/ygelfand/libcountertop/pkg/hook"
)

func init() {
	component.Register(component.Device, Get, component.Order(45))
}

const (
	// frame is how often the panel is redrawn while it is up. The wave is the reason: a turn is
	// seconds long and this is the whole of what the device is doing, so it is worth the frames.
	frame = 50 * time.Millisecond

	// slide is how long the panel takes to come down, and to go back up.
	slide = 220 * time.Millisecond

	// reading is how fast a reply is revealed when nothing says how long the speech will take, in
	// characters a second. Ordinary speech is around fifteen.
	reading = 15.0
)

type Assistant struct {
	Frame hook.Hook[Showing]

	mu sync.Mutex

	// turn is the last thing the conversation said, and since is when it said it. The reveal is
	// paced from since, so it starts when the reply does rather than when the panel opened.
	turn  voice.Showing
	since time.Time

	// opened is when the panel started sliding down, and closing is when it started going back.
	opened  time.Time
	closing time.Time

	// wake is how the drawing loop is told something changed, so it is not left redrawing a panel
	// that is not up.
	wake chan struct{}
	took int
}

var (
	once   sync.Once
	shared *Assistant
)

func Get() *Assistant {
	once.Do(func() {
		shared = &Assistant{wake: make(chan struct{}, 1)}

		voice.Shown.Listen(shared.show)

		// While a turn is up, touching the screen ends it without talking — the same thing the
		// action button does on a device that has one, which this does not.
		touch.Get().Contacts.Listen(func(c touch.Contact) {
			if c.Phase == touch.Down {
				shared.cancel(c.ID)
			}
		})
	})
	return shared
}

func (a *Assistant) Name() string { return "assistant screen" }

// show takes what the conversation is doing now.
func (a *Assistant) show(s voice.Showing) {
	a.mu.Lock()

	// Only the reply restarts the clock the reveal is paced from. A phase change during one — the
	// audio starting after the text arrived — must not send it back to the beginning.
	if s.Reply != a.turn.Reply || a.turn.Phase == voice.Idle {
		a.since = time.Now()
	}
	if a.turn.Phase == voice.Idle && s.Phase != voice.Idle {
		a.opened, a.closing = time.Now(), time.Time{}
	}
	if s.Phase == voice.Idle && a.turn.Phase != voice.Idle {
		a.closing = time.Now()
	}
	a.turn = s
	a.mu.Unlock()

	select {
	case a.wake <- struct{}{}:
	default:
	}
}

// Run draws the panel while there is one, and waits to be woken while there is not.
func (a *Assistant) Run(ctx context.Context) error {
	for {
		if !a.showing() {
			a.hide()

			select {
			case <-ctx.Done():
				return nil
			case <-a.wake:
			}
			continue
		}

		a.paint()

		select {
		case <-ctx.Done():
			return nil
		case <-a.wake:
		case <-time.After(frame):
		}
	}
}

// showing is whether there is anything to draw, which outlasts the turn by however long the panel
// takes to slide back up.
func (a *Assistant) showing() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.turn.Phase != voice.Idle {
		return true
	}
	return !a.closing.IsZero() && time.Since(a.closing) < slide
}

// paint draws one frame.
func (a *Assistant) Now() (Showing, bool) {
	now := time.Now()

	a.mu.Lock()
	show := Showing{
		Showing: a.turn,
		At:      now,
		Down:    a.down(now),
		Reveal:  a.reveal(now),
	}
	a.mu.Unlock()

	if show.Phase == voice.Listening {
		show.Level = mic.Get().Level()
	}
	return show, show.Down > 0
}

func (a *Assistant) paint() {
	show, _ := a.Now()
	a.Frame.Emit(show)
}

// down is how far out the panel is. Held with mu.
func (a *Assistant) down(now time.Time) float64 {
	if a.turn.Phase == voice.Idle {
		if a.closing.IsZero() {
			return 0
		}
		return 1 - min(now.Sub(a.closing).Seconds()/slide.Seconds(), 1)
	}
	if a.opened.IsZero() {
		return 1
	}
	return min(now.Sub(a.opened).Seconds()/slide.Seconds(), 1)
}

// reveal is how much of the reply has been spoken. Held with mu.
//
// Paced against a reading speed rather than against the audio. The length of a whole-file reply is
// known only once it has been fetched and a streamed one has no length at all until it ends, so
// there is nothing to pace against that is right for both — and a reveal that runs slightly ahead
// of the voice reads better than one that stalls waiting for it.
func (a *Assistant) reveal(now time.Time) float64 {
	if a.turn.Reply == "" || a.turn.Phase != voice.Replying {
		return 0
	}

	runes := float64(len([]rune(a.turn.Reply)))
	if runes == 0 {
		return 1
	}
	return min(now.Sub(a.since).Seconds()*reading/runes, 1)
}

func (a *Assistant) hide() { a.Frame.Emit(Showing{}) }

// cancel ends the turn a touch landed on. A touch with no turn up is somebody else's.
func (a *Assistant) Took(id int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.took == id+1
}

func (a *Assistant) cancel(id int) {
	a.mu.Lock()
	showing := a.turn.Phase != voice.Idle
	a.took = 0
	if showing {
		a.took = id + 1
	}
	a.mu.Unlock()

	if !showing {
		return
	}
	if voice.Get().Stop() {
		slog.Info("turn ended by touch")
	}
}
