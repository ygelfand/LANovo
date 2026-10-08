package sensors

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/hook"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/i2c"
	"github.com/ygelfand/LANovo/internal/hardware/light"
	"github.com/ygelfand/LANovo/internal/hardware/motion"
	"github.com/ygelfand/LANovo/internal/service"
)

func init() {
	component.Register(component.Hardware, Get, component.Order(40),
		component.Supervise(service.Restart(5*time.Second, time.Minute)))
}

const interval = 250 * time.Millisecond

// The parts' interrupt lines are not wired anywhere reachable.
const (
	change = 0.05

	leastLux  = 15
	leastNear = 4

	stale = 5 * time.Minute
)

const settle = 0.25

type Sensors struct {
	Turned hook.Hook[display.Orientation]

	Arrived hook.Hook[bool]
	seen    atomic.Int64

	lux       *esphome.Sensor
	proximity *esphome.Sensor
	present   *esphome.BinarySensor
	facing    *esphome.TextSensor
	nearby    presence

	toldLux  told
	toldNear told

	following float64
	followed  bool
	held      float64
	shown     int
	applied   int
	ambient   atomic.Uint64
	lit       atomic.Bool

	mu      sync.Mutex
	bus     i2c.Bus
	light   *light.Sensor
	motion  *motion.Sensor
	tracker *motion.Tracker
}

type told struct {
	value float64
	at    time.Time
	sent  bool
}

func (t *told) worth(v, least float64) bool {
	now := time.Now()

	switch {
	case !t.sent, now.Sub(t.at) >= stale:
	case math.Abs(v-t.value) >= max(least, math.Abs(t.value)*change):
	default:
		return false
	}

	t.value, t.at, t.sent = v, now, true
	return true
}

var (
	once   sync.Once
	shared *Sensors
)

func Get() *Sensors {
	once.Do(func() {
		shared = &Sensors{tracker: motion.NewTracker()}
		shared.build()
	})
	return shared
}

func (s *Sensors) Name() string { return "sensors" }

func (s *Sensors) Entities() []esphome.Entity {
	return append(
		[]esphome.Entity{s.lux, s.present, s.proximity, s.facing},
		controls().Entities()...)
}

func (s *Sensors) Restore(config.Config) { controls().Publish() }

func (s *Sensors) Seen() (time.Duration, bool) {
	at := s.seen.Load()
	if at == 0 {
		return 0, false
	}
	return time.Since(time.Unix(0, at)), true
}

func (s *Sensors) Orientation() display.Orientation { return s.tracker.Orientation() }

func (s *Sensors) Ambient() (lux float64, ok bool) {
	return math.Float64frombits(s.ambient.Load()), s.lit.Load()
}

func (s *Sensors) Turn(rot display.Orientation) {
	slog.Info("turned by hand", "orientation", rot)

	display.Get().SetOrientation(rot)
	s.facing.Set(rot.String())
	s.Turned.Emit(rot)
}

func (s *Sensors) Orient() {
	s.resetMounted()
	if err := s.Start(context.Background()); err != nil {
		slog.Warn("could not read the orientation before drawing", "err", err)
		return
	}

	s.mu.Lock()
	m := s.motion
	s.mu.Unlock()

	if m == nil {
		return
	}

	r, err := m.Read()
	if err != nil {
		slog.Warn("could not read the accelerometer before drawing", "err", err)
		return
	}

	s.tracker.Update(r)
	rot := s.tracker.Orientation()

	display.Get().SetOrientation(rot)
	s.facing.Set(rot.String())
	slog.Info("standing", "orientation", rot)
}

func (s *Sensors) resetMounted() {
	s.mu.Lock()
	s.tracker = motion.NewTracker()
	rot := s.tracker.Orientation()
	s.mu.Unlock()
	display.Get().SetOrientation(rot)
	s.facing.Set(rot.String())
}

func (s *Sensors) Start(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.bus != nil {
		return nil
	}

	if l, bus, err := light.Open(); err != nil {
		slog.Warn("no ambient light sensor", "err", err)
	} else {
		s.light, s.bus = l, bus
	}

	if !board.Current().Motion {
		return nil
	}
	if m, bus, err := motion.Open(); err != nil {
		slog.Warn("no accelerometer", "err", err)
	} else {
		s.motion = m
		if s.bus == nil {
			s.bus = bus
		}
	}
	return nil
}

func (s *Sensors) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.bus == nil {
		return nil
	}
	err := s.bus.Close()
	s.bus, s.light, s.motion = nil, nil, nil
	return err
}

func (s *Sensors) Run(ctx context.Context) error {
	tick := time.NewTicker(interval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}

		s.follow()
		s.publish()
	}
}

func (s *Sensors) follow() {
	s.mu.Lock()
	m := s.motion
	s.mu.Unlock()

	if m == nil {
		return
	}

	r, err := m.Read()
	if err != nil {
		slog.Warn("reading the accelerometer failed", "err", err)
		return
	}
	if !s.tracker.Update(r) {
		return
	}

	rot := s.tracker.Orientation()
	slog.Info("turned", "orientation", rot)

	display.Get().SetOrientation(rot)
	s.facing.Set(rot.String())
	s.Turned.Emit(rot)
}

func (s *Sensors) publish() {
	s.mu.Lock()
	l := s.light
	s.mu.Unlock()

	if l == nil {
		return
	}

	if lux, err := l.Lux(); err != nil {
		slog.Warn("reading the ambient light failed", "err", err)
	} else {
		s.autoBacklight(lux)

		if s.toldLux.worth(s.following, leastLux) {
			s.lux.Set(float32(s.following))
		}
	}

	if near, err := l.Proximity(); err != nil {
		slog.Warn("reading proximity failed", "err", err)
	} else {
		if s.toldNear.worth(float64(near), leastNear) {
			s.proximity.Set(float32(near))
		}
		now := time.Now()
		here, changed := s.nearby.update(float64(near), rise(config.Get().Presence.Range), now)
		if here {
			s.seen.Store(now.UnixNano())
		}
		if changed {
			slog.Debug("presence", "near", here, "counts", near, "baseline", int(s.nearby.base))
			s.present.Set(here)
			s.Arrived.Emit(here)
		}
	}
}

func (s *Sensors) autoBacklight(lux float64) {
	if !s.followed {
		s.following, s.followed, s.held = lux, true, lux
	}
	s.following += settle * (lux - s.following)
	s.ambient.Store(math.Float64bits(s.following))
	s.lit.Store(true)

	cfg := config.Get()
	if cfg.Screen.Mode != config.ModeAuto {
		s.applied = 0
		return
	}

	s.held = hold(s.held, s.following, lux)
	want := Brightness(s.held, cfg.Screen.Backlight)
	if s.shown == 0 {
		s.shown = want
	}
	if s.shown = ramp(s.shown, want); s.shown == want && want == s.applied {
		return
	}
	want = s.shown
	s.applied = want
	if err := display.Get().Brightness(want); err != nil {
		slog.Warn("setting the backlight failed", "err", err)
		return
	}

	screen.Get().Lit(want)
}

func (s *Sensors) build() {
	s.lux = &esphome.Sensor{
		Base: esphome.Base{
			ObjectID: "ambient_light", Name: "Ambient light", Icon: "mdi:brightness-5",
		},
		Unit:        "lx",
		DeviceClass: "illuminance",
		StateClass:  esphome.StateClassMeasurement,
		Decimals:    1,
	}

	s.proximity = &esphome.Sensor{
		Base: esphome.Base{
			ObjectID: "proximity", Name: "Proximity", Icon: "mdi:motion-sensor",
			Category: esphome.CategoryDiagnostic,
		},
		StateClass: esphome.StateClassMeasurement,
	}

	s.present = &esphome.BinarySensor{
		Base: esphome.Base{
			ObjectID: "presence", Name: "Presence", Icon: "mdi:account",
		},
		DeviceClass: "occupancy",
	}

	s.facing = &esphome.TextSensor{
		Base: esphome.Base{
			ObjectID: "orientation", Name: "Orientation", Icon: "mdi:screen-rotation",
			Category: esphome.CategoryDiagnostic,
		},
	}
	s.facing.Set(s.tracker.Orientation().String())
}
