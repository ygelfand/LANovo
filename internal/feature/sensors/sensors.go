package sensors

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/display/brightness"
	"github.com/ygelfand/libcountertop/pkg/display/geometry"
	"github.com/ygelfand/libcountertop/pkg/hook"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/i2c"
	"github.com/ygelfand/LANovo/internal/hardware/light"
	"github.com/ygelfand/LANovo/internal/hardware/motion"
)

func init() {
	component.Register(sharedcomponent.Hardware, Get, sharedcomponent.Order(40),
		sharedcomponent.Supervise(service.Restart(5*time.Second, time.Minute)))
}

const interval = 250 * time.Millisecond

// The parts' interrupt lines are not wired anywhere reachable.
const (
	leastLux  = 15
	leastNear = 4
)

type Sensors struct {
	Turned hook.Hook[geometry.Orientation]

	Arrived hook.Hook[bool]
	seen    atomic.Int64

	lux       *esphome.Sensor
	proximity *esphome.Sensor
	present   *esphome.BinarySensor
	facing    *esphome.TextSensor
	nearby    presence

	toldNear brightness.Told
	auto     *brightness.Auto

	mu      sync.Mutex
	bus     i2c.Bus
	light   *light.Sensor
	motion  *motion.Sensor
	tracker *motion.Tracker
}

var (
	once   sync.Once
	shared *Sensors
)

func Get() *Sensors {
	once.Do(func() {
		shared = &Sensors{tracker: motion.NewTracker(), auto: brightness.NewAuto(leastLux)}
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

func (s *Sensors) Orientation() geometry.Orientation { return s.tracker.Orientation() }

func (s *Sensors) Ambient() (lux float64, ok bool) { return s.auto.Ambient() }

func (s *Sensors) Turn(rot geometry.Orientation) {
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
	}

	if near, err := l.Proximity(); err != nil {
		slog.Warn("reading proximity failed", "err", err)
	} else {
		if s.toldNear.Worth(float64(near), leastNear) {
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
	step := s.auto.Follow(lux, config.Get().Screen)
	if step.Report {
		s.lux.Set(float32(step.Ambient))
	}
	if !step.Apply {
		return
	}
	if err := display.Get().Brightness(step.Level); err != nil {
		slog.Warn("setting the backlight failed", "err", err)
		return
	}

	screen.Get().Lit(step.Level)
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
