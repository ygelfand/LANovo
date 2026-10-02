package youtube

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/lib/cast"
)

// Retry is how long to wait after the lounge could not be reached or bound.
const Retry = 30 * time.Second

const firstRetry = 2 * time.Second

// themes is which lounge screen each YouTube application pairs through.
var themes = map[string]string{
	"2DB7CC49": ThemeMusic,
	"233637DE": ThemeYouTube,
	"32EAB1DF": ThemeTV,
}

// screens keeps this device's lounge screens bound, one per theme, with their ids in the config.
type screens struct {
	opts cast.Env

	mu       sync.Mutex
	id       config.YouTube
	ctx      context.Context
	stations map[string]*station
	live     map[string]bool
}

const Idle = 10 * time.Minute

func newScreens(opts cast.Env) *screens {
	s := &screens{opts: opts, id: config.Get().Cast.YouTube}
	if s.id.Device == "" {
		s.id.Device = uuid()
		s.save()
	}
	return s
}

// screen is the lounge screen an application pairs through, once YouTube has issued one.
func (s *screens) screen(app string) (string, string, bool) {
	theme, ok := themes[app]
	if !ok {
		return "", "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.screenID(theme)
	return id, s.id.Device, id != ""
}

func (s *screens) screenID(theme string) string {
	if theme == ThemeMusic {
		return s.id.Music
	}
	return s.id.Video
}

func (s *screens) setScreenID(theme, id string) {
	s.mu.Lock()
	if theme == ThemeMusic {
		s.id.Music = id
	} else {
		s.id.Video = id
	}
	s.mu.Unlock()
	s.save()
}

func (s *screens) save() {
	s.mu.Lock()
	id := s.id
	s.mu.Unlock()
	if err := config.Set().Cast().Screens(id.Device, id.Music, id.Video); err != nil {
		slog.Warn("saving the youtube screens", "err", err)
	}
}

// run keeps every theme's screen bound until ctx ends.
func (s *screens) run(ctx context.Context) {
	s.mu.Lock()
	s.ctx = ctx
	s.stations = map[string]*station{}
	s.live = map[string]bool{}
	onDemand := config.Get().Cast.YouTube.OnDemand
	for _, theme := range []string{ThemeMusic, ThemeYouTube, ThemeTV} {
		s.stations[theme] = newStation(ctx, theme, s.opts)
	}
	s.mu.Unlock()
	slog.Info("youtube lounge", "mode", map[bool]string{false: "always", true: "on demand"}[onDemand])
	if onDemand {
		return
	}
	for _, theme := range []string{ThemeMusic, ThemeYouTube} {
		s.start(theme, ctx)
	}
}

func (s *screens) wake(app string) {
	theme, ok := themes[app]
	if !ok {
		return
	}
	s.mu.Lock()
	st, ctx, live := s.stations[theme], s.ctx, s.live[theme]
	s.mu.Unlock()
	if st == nil {
		return
	}
	st.markAsked(app)
	if !live {
		lctx, cancel := context.WithCancel(ctx)
		s.start(theme, lctx)
		go s.idle(lctx, cancel, st)
		return
	}
	go st.report(st.state())
}

func (s *screens) start(theme string, ctx context.Context) {
	s.mu.Lock()
	st := s.stations[theme]
	s.live[theme] = true
	s.mu.Unlock()
	go func() {
		s.keep(ctx, st)
		s.mu.Lock()
		s.live[theme] = false
		s.mu.Unlock()
	}()
}

func (s *screens) idle(ctx context.Context, cancel context.CancelFunc, st *station) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if st.quiet(Idle) {
				slog.Info("youtube lounge let go", "theme", st.theme, "quiet for", Idle)
				cancel()
				return
			}
		}
	}
}

// keep binds one screen and holds its long poll, binding again whenever the session goes.
func (s *screens) keep(ctx context.Context, st *station) {
	delay := firstRetry
	for ctx.Err() == nil {
		err := s.bound(ctx, st)
		if err == nil {
			delay = firstRetry
			continue
		}
		if ctx.Err() == nil {
			slog.Warn("youtube lounge", "theme", st.theme, "err", err, "again in", delay)
			wait(ctx, delay)
			delay = min(2*delay, Retry)
		}
	}
}

func (s *screens) bound(ctx context.Context, st *station) error {
	theme := st.theme
	s.mu.Lock()
	id := s.screenID(theme)
	device := s.id.Device
	s.mu.Unlock()

	if id == "" {
		fresh, err := NewScreenID(ctx, s.opts.HTTP)
		if err != nil {
			return fmt.Errorf("a screen id: %w", err)
		}
		s.setScreenID(theme, fresh)
		id = fresh
		slog.Info("youtube lounge screen issued", "theme", theme, "screen", id)
	}

	token, err := LoungeToken(ctx, s.opts.HTTP, id)
	if err != nil {
		return fmt.Errorf("a lounge token: %w", err)
	}

	screen := Screen{ID: id, Device: device, Name: s.opts.Name, Theme: theme, Brand: s.opts.Model, Model: s.opts.Model}
	sess, err := Bind(ctx, s.opts.HTTP, s.opts.Long, screen, token)
	if err != nil {
		return fmt.Errorf("binding: %w", err)
	}
	slog.Info("youtube lounge bound", "theme", theme, "screen", id, "since asked", st.sinceAsked())
	st.bind(sess)
	st.touch()
	st.report(st.state())
	st.discovery()

	for ctx.Err() == nil {
		err := sess.Poll(ctx, func(m Message) {
			if m.Name == "noop" {
				return
			}
			slog.Info("youtube lounge in", "theme", theme, "aid", m.AID, "name", m.Name, "payload", string(m.Payload))
			st.touch()
			st.handle(m)
		})
		if errors.Is(err, ErrExpired) || time.Now().After(token.Expires) {
			return nil
		}
		if errors.Is(err, ErrQuiet) {
			slog.Warn("youtube lounge poll went quiet", "theme", theme, "after", Quiet)
			continue
		}
		if err != nil && ctx.Err() == nil {
			return fmt.Errorf("polling: %w", err)
		}
	}
	return nil
}

func wait(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func uuid() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
