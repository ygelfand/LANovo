package timer

import (
	"cmp"
	"context"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/go-esphome-device/api"
	sharedtone "github.com/ygelfand/libcountertop/pkg/audio/tone"
	"github.com/ygelfand/libcountertop/pkg/hook"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func init() {
	component.Register(component.Device, Get, component.Order(31))
}

const (
	refresh = 250 * time.Millisecond

	ringFor   = 15 * time.Minute
	ringEvery = 2 * time.Second

	alarmLevel = 0.6
)

type Timers struct {
	Changed hook.Hook[Card]

	names *esphome.TextSensor

	woke chan struct{}

	mu    sync.Mutex
	held  map[string]*timer
	stop  context.CancelFunc
	shown string
}

type timer struct {
	name   string
	total  time.Duration
	left   time.Duration
	at     time.Time
	active bool
}

func (t *timer) remaining(now time.Time) time.Duration {
	if !t.active {
		return t.left
	}
	return max(t.left-now.Sub(t.at), 0)
}

var (
	once   sync.Once
	shared *Timers
)

func Get() *Timers {
	once.Do(func() { shared = build() })
	return shared
}

func build() *Timers {
	return &Timers{
		names: &esphome.TextSensor{
			Base: esphome.Base{
				ObjectID: "timers",
				Name:     "Timers",
				Icon:     "mdi:timer-outline",
				Category: esphome.CategoryDiagnostic,
			},
		},
		woke: make(chan struct{}, 1),
		held: map[string]*timer{},
	}
}

func (t *Timers) Name() string { return "timers" }

func (t *Timers) Entities() []esphome.Entity { return []esphome.Entity{t.names} }

func (t *Timers) Run(ctx context.Context) error {
	for {
		if !t.counting() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.woke:
			}
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(refresh):
		}
		t.show()
	}
}

func (t *Timers) counting() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.soonest(time.Now()) != nil
}

func (t *Timers) Event(e esphome.TimerEvent) {
	slog.Debug(
		"timer",
		"event",
		e.Type,
		"name",
		e.Name,
		"left",
		e.SecondsLeft,
		"total",
		e.TotalSeconds,
		"active",
		e.IsActive,
	)

	switch e.Type {
	case api.VoiceAssistantTimerEvent_VOICE_ASSISTANT_TIMER_STARTED,
		api.VoiceAssistantTimerEvent_VOICE_ASSISTANT_TIMER_UPDATED:
		t.set(e)
	case api.VoiceAssistantTimerEvent_VOICE_ASSISTANT_TIMER_CANCELLED:
		t.forget(e.TimerID)
	case api.VoiceAssistantTimerEvent_VOICE_ASSISTANT_TIMER_FINISHED:
		t.finished(e)
	}
	t.show()
	t.publish()
}

func (t *Timers) publish() {
	t.mu.Lock()
	now := time.Now()
	running := make([]*timer, 0, len(t.held))
	for _, c := range t.held {
		if c.active {
			running = append(running, c)
		}
	}
	t.mu.Unlock()

	slices.SortFunc(
		running,
		func(a, b *timer) int { return cmp.Compare(a.remaining(now), b.remaining(now)) },
	)

	names := make([]string, 0, len(running))
	for _, c := range running {
		names = append(names, cmp.Or(c.name, "Timer"))
	}
	t.names.Set(strings.Join(names, ", "))
}

func (t *Timers) Forget() {
	t.mu.Lock()
	n := len(t.held)
	t.held = map[string]*timer{}
	t.mu.Unlock()

	if n > 0 {
		slog.Info("timers forgotten", "count", n)
	}
	t.show()
	t.publish()
}

func (t *Timers) Ringing() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stop != nil
}

func (t *Timers) Stop() bool {
	t.mu.Lock()
	stop := t.stop
	t.mu.Unlock()

	if stop == nil {
		return false
	}
	stop()
	return true
}

func (t *Timers) set(e esphome.TimerEvent) {
	t.mu.Lock()
	t.held[e.TimerID] = &timer{
		name:   e.Name,
		total:  time.Duration(e.TotalSeconds) * time.Second,
		left:   time.Duration(e.SecondsLeft) * time.Second,
		at:     time.Now(),
		active: e.IsActive,
	}
	t.mu.Unlock()

	select {
	case t.woke <- struct{}{}:
	default:
	}
}

func (t *Timers) forget(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.held, id)
}

func (t *Timers) finished(e esphome.TimerEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.held, e.TimerID)
	if t.stop != nil {
		return
	}

	var ctx context.Context
	ctx, t.stop = context.WithCancel(context.Background())
	safe.Go("timer alarm", func() { t.ring(ctx) })
}

func (t *Timers) ring(ctx context.Context) {
	defer func() {
		t.mu.Lock()
		t.stop = nil
		t.mu.Unlock()
		t.show()
	}()

	sound := speaker.Sound()
	sound.Backgrounds().Duck(true)
	defer sound.Backgrounds().Duck(false)

	over := time.After(ringFor)
	for {
		sound.Interject(func(p *speaker.Speaker) { p.Chime(alarmLevel, sharedtone.ToneTimer...) })

		select {
		case <-ctx.Done():
			return
		case <-over:
			slog.Info("timer rang out", "for", ringFor)
			return
		case <-time.After(ringEvery):
		}
	}
}

func (t *Timers) Now() Card {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.now()
}

func (t *Timers) now() Card {
	now := time.Now()
	card := Card{Ringing: t.stop != nil}
	if soon := t.soonest(now); soon != nil {
		card.Name, card.Left, card.Of = soon.name, soon.remaining(now), soon.total
		card.Showing = true
	}

	card.Showing = card.Showing || card.Ringing
	return card
}

func (t *Timers) show() {
	t.mu.Lock()
	card := t.now()
	if same := card.key(); same == t.shown {
		t.mu.Unlock()
		return
	} else {
		t.shown = same
	}
	t.mu.Unlock()
	t.Changed.Emit(card)
}

func (t *Timers) soonest(now time.Time) *timer {
	var soon *timer
	for _, c := range t.held {
		if !c.active {
			continue
		}
		if soon == nil || c.remaining(now) < soon.remaining(now) {
			soon = c
		}
	}
	return soon
}
