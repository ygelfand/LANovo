package gui

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/assistant"
	"github.com/ygelfand/LANovo/internal/feature/firmware"
	"github.com/ygelfand/LANovo/internal/feature/message"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/timer"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/touch"
	"github.com/ygelfand/LANovo/internal/lib/surface"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	backend "github.com/ygelfand/libcountertop/pkg/display/gogui"
	interaction "github.com/ygelfand/libcountertop/pkg/display/interaction"
	navigation "github.com/ygelfand/libcountertop/pkg/display/navigation"
	"github.com/ygelfand/libcountertop/pkg/display/widgets"
)

const (
	layerID    = 60
	layerZ     = 5
	helperWait = 200 * time.Millisecond
)

type App struct {
	mu  sync.Mutex
	w   *gogui.Window
	r   *backend.Renderer
	nav *Nav
}

var (
	once   sync.Once
	shared *App
)

func init() {
	component.Register(component.Device, Get, component.Order(36))
}

func Get() *App {
	once.Do(func() {
		shared = &App{}
		shared.nav = &Nav{For: shared.screenFor}
	})
	return shared
}

func (a *App) Name() string { return "gui" }

func sizeSetting() string {
	if s := config.Get().Screen.Size; s != "" {
		return s
	}
	if s := board.Current().UISize; s != "" {
		return s
	}
	return SizeLarge
}

func current() gogui.Theme {
	s := config.Get().Screen
	return Look(s.Style, s.Theme, sizeSetting())
}

func (a *App) Restyle() {
	a.mu.Lock()
	w := a.w
	a.mu.Unlock()
	if w != nil {
		w.QueueCommand(func(w *gogui.Window) { w.SetTheme(current()) })
	}
}

var phases = map[touch.Phase]backend.Phase{touch.Down: backend.Began, touch.Move: backend.Moved, touch.Up: backend.Ended}

var backSweep = navigation.BackSweep

func afterBoot(ctx context.Context) bool {
	done := make(chan struct{}, 1)
	stop := display.Get().Released.Listen(func(p display.Priority) {
		if p == display.PriorityBoot {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})
	defer stop()
	for display.Get().Held(display.PriorityBoot) {
		select {
		case <-ctx.Done():
			return false
		case <-done:
		}
	}
	return true
}

func (a *App) Run(ctx context.Context) error {
	defer func() {
		if playerView != nil {
			playerView.Close()
		}
	}()
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	var c *surface.Client
	for c = display.Get().Helper(); c == nil || c.Err() != nil; c = display.Get().Helper() {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(helperWait):
		}
	}
	if !afterBoot(ctx) {
		return nil
	}
	nw, nh := display.Get().Native()
	if err := c.UIOpen(layerID, nw, nh, layerZ); err != nil {
		return fmt.Errorf("gui: opening the UI layer: %w", err)
	}
	defer func() { stop(); _ = c.VideoClose(layerID) }()

	rot := display.Get().Orientation()
	vw, vh := rot.Size(nw, nh)
	gogui.SetTheme(current())
	interactions = interaction.New()
	editor = widgets.NewEditor()
	cameraPreview = &widgets.LivePreview{}
	gestures = &widgets.Gestures{Release: release}
	w := gogui.SimpleWindow("lanovo", vw, vh, a, func(w *gogui.Window) { w.SetView(a.root) })
	r, err := backend.New(surface.UILayer{C: c, ID: layerID}, w)
	if err != nil {
		return err
	}
	r.SetRotation(int(rot) / 90)

	a.mu.Lock()
	a.w, a.r = w, r
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.w, a.r = nil, nil
		a.mu.Unlock()
	}()

	var width atomic.Int32
	width.Store(int32(vw))
	type start struct {
		at   [2]int
		open bool
	}
	starts := map[int]start{}

	stopTouch := touch.Get().Contacts.Listen(func(t touch.Contact) {
		shell.Get().Touch()
		if message.Get().Took(t.ID) || assistant.Get().Took(t.ID) {
			return
		}
		p, ok := phases[t.Phase]
		if !ok {
			return
		}
		switch t.Phase {
		case touch.Down:
			starts[t.ID] = start{at: [2]int{t.X, t.Y}, open: shell.Get().Open()}
		case touch.Up:
			from, seen := starts[t.ID]
			delete(starts, t.ID)
			if seen && from.open && backSweep(from.at, t.X, t.Y, int(width.Load())) {
				r.Touch(backend.Cancelled, uint64(t.ID), float32(t.X), float32(t.Y))
				w.QueueCommand(func(w *gogui.Window) {
					shell.Get().Pop()
					w.InvalidateLayout()
				})
				return
			}
		}
		r.Touch(p, uint64(t.ID), float32(t.X), float32(t.Y))
		if t.Phase == touch.Up {
			w.QueueCommand(func(w *gogui.Window) {
				lift(w)
				if !typing(w) {
					w.ClearFocus()
				}
			})
		}
	})
	defer stopTouch()

	var look atomic.Value
	look.Store(current().Cfg.Name)
	restyle := func() {
		if now := current().Cfg.Name; now != look.Load() {
			look.Store(now)
			a.Restyle()
		}
	}
	stopStyle := screen.Get().Restyled.Listen(func(config.Screen) { restyle() })
	defer stopStyle()
	stopTheme := screen.Get().Themed.Listen(func(theme.Theme) { restyle() })
	defer stopTheme()

	stopTurn := sensors.Get().Turned.Listen(func(o display.Orientation) {
		w.QueueCommand(func(w *gogui.Window) {
			r.SetRotation(int(o) / 90)
			ww, hh := o.Size(nw, nh)
			width.Store(int32(ww))
			w.EventFn(&gogui.Event{Type: gogui.EventResized, WindowWidth: ww, WindowHeight: hh})
		})
	})
	defer stopTurn()

	redraw := func() { w.QueueCommand(func(w *gogui.Window) { w.InvalidateLayout() }) }

	stopShell := shell.Get().Redrawn.Listen(func(struct{}) { redraw() })
	defer stopShell()
	stopMoved := shell.Get().Changed.Listen(func(shell.Change) {
		w.QueueCommand(func(w *gogui.Window) {
			editor.Shift, editor.Symbols = false, false
			w.ClearFocus()
		})
	})
	defer stopMoved()
	for _, o := range []*Overlay{{Priority: priorityAlert, Build: messageCard}, {Priority: priorityNotice, Build: timerCard}, {Priority: priorityNotice + 1, Build: assistantPanel}, {Priority: priorityMini, Build: a.mini}, {Priority: priorityMarks, Build: privacyMarks}, {Priority: priorityUpgrade, Build: upgradeCard}} {
		a.nav.Show(o)
		defer a.nav.Hide(o)
	}
	stopMessage := message.Get().Changed.Listen(func(bool) { redraw() })
	defer stopMessage()
	var spinning atomic.Bool
	stopUpgrade := firmware.Get().Upgrading.Listen(func(up firmware.Upgrade) {
		redraw()
		if !up.Active() || !spinning.CompareAndSwap(false, true) {
			return
		}
		go func() {
			defer spinning.Store(false)
			for firmware.Get().Upgrade().Active() {
				redraw()
				select {
				case <-ctx.Done():
					return
				case <-time.After(upgradeFrame):
				}
			}
			redraw()
		}()
	})
	defer stopUpgrade()
	stopTimer := timer.Get().Changed.Listen(func(timer.Card) { redraw() })
	defer stopTimer()
	stopAssistant := assistant.Get().Frame.Listen(func(assistant.Showing) { redraw() })
	defer stopAssistant()
	stopMarks := privacy.Get().Changed.Listen(func(privacy.Marks) { redraw() })
	stopOffer := web.Get().Offered.Listen(func(string) { redraw() })
	defer stopOffer()
	defer stopMarks()

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				redraw()
			}
		}
	}()

	slog.Info("gui: running", "size", fmt.Sprintf("%dx%d", vw, vh), "theme", current().Cfg.Name)
	return r.Run(ctx, w)
}

// Go Bold declares OS/2 usWeightClass 600; go-glyph files only 700 and up as bold.
