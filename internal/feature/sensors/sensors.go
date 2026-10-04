// Package sensors reads what the room is doing and says so.
//
// Ambient light, proximity and which way up the device is. All three come from parts Lenovo's app
// drove as Android Things user drivers, so nothing else is reading them.
package sensors

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/i2c"
	"github.com/ygelfand/LANovo/internal/hardware/light"
	"github.com/ygelfand/LANovo/internal/hardware/motion"
	"github.com/ygelfand/LANovo/internal/lib/hook"
	"github.com/ygelfand/LANovo/internal/service"
)

func init() {
	component.Register(component.Hardware, Get, component.Order(40),
		component.Supervise(service.Restart(5*time.Second, time.Minute)))
}

// interval is how often the parts are read. The accelerometer decides how quickly the screen
// follows the device being turned, which is the fastest thing here that matters.
const interval = 250 * time.Millisecond

// A reading reaches Home Assistant when it has actually moved, rather than on a timer. Everything
// is read every interval either way — the parts are on a bus that is already being talked to, and
// their interrupt lines go nowhere we can reach — so the only question is what is worth saying.
const (
	// change is how far a reading must move from the last one sent, as a fraction of it.
	change = 0.05

	// Below this a reading has not really moved, whatever the fraction says: five percent of a
	// nearly dark room is under the part's own noise, and would report on every tick.
	//
	// The part wanders about twelve counts end to end at rest, which through the glass is around
	// ten lux of room, so this clears the noise with a little to spare. It only decides matters
	// below three hundred lux; above that the fraction is larger.
	leastLux  = 15
	leastNear = 4

	// stale is the longest Home Assistant goes without hearing anything. Nothing needs it, but its
	// hourly statistics are thin if a still room says nothing for an afternoon.
	stale = 5 * time.Minute
)

// settle is how much of each reading moves the level the backlight follows, which at four readings
// a second lands a real change within about a second. Without it a hand passing the sensor steps
// the panel, and brightness that jumps reads worse than brightness that lags.
const settle = 0.25

type Sensors struct {
	// Turned carries every change in which way up the device is.
	Turned hook.Hook[display.Orientation]

	Arrived hook.Hook[bool]
	seen    atomic.Int64

	lux       *esphome.Sensor
	proximity *esphome.Sensor
	present   *esphome.BinarySensor
	facing    *esphome.TextSensor
	nearby    presence

	// What Home Assistant was last told, so it only hears about a reading that has moved.
	toldLux  told
	toldNear told

	// following is the smoothed light the backlight tracks, and whether it has a value yet.
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

// told is the last reading handed to Home Assistant, and when.
type told struct {
	value float64
	at    time.Time
	sent  bool
}

// worth reports whether a reading differs enough from the last one sent to be worth sending, and
// remembers it when it does. Comparing against what was sent rather than what was last read is
// what stops a reading sitting on the boundary from reporting every tick.
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
	return append([]esphome.Entity{s.lux, s.present, s.proximity, s.facing}, controls().Entities()...)
}

func (s *Sensors) Restore(config.Config) { controls().Publish() }

func (s *Sensors) Seen() (time.Duration, bool) {
	at := s.seen.Load()
	if at == 0 {
		return 0, false
	}
	return time.Since(time.Unix(0, at)), true
}

// Orientation is which way up the device is.
func (s *Sensors) Orientation() display.Orientation { return s.tracker.Orientation() }

func (s *Sensors) Ambient() (lux float64, ok bool) {
	return math.Float64frombits(s.ambient.Load()), s.lit.Load()
}

// Turn faces the picture a way nobody moved the device to.
//
// For the harness: which way up the device is stands behind a lot of the drawing, and checking that
// by hand means somebody standing over it. The tracker is deliberately left alone, so the next real
// movement corrects this — a lie told to the display rather than to the sensor, which is the kind
// the accelerometer can overrule.
func (s *Sensors) Turn(rot display.Orientation) {
	slog.Info("turned by hand", "orientation", rot)

	display.Get().SetOrientation(rot)
	s.facing.Set(rot.String())
	s.Turned.Emit(rot)
}

// Orient reads which way up the device is and turns the screen to match, before anything is drawn.
//
// Called at boot rather than left to the reading loop: the panel opens at the rotation the device
// is usually stood in, and a logo that appears and then flips a moment later is the device telling
// you it was not looking.
func (s *Sensors) Orient() {
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

// Start finds the parts. A board missing one is not a board that cannot run: each is reported and
// left out. Opening twice is a no-op, since Orient opens before the supervisor does.
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

// Run reads the parts until ctx is canceled.
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

// follow turns the screen when the device is turned.
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

// publish reads the parts and passes on whatever has moved.
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

		// The smoothed level rather than the reading, which at four readings a second wanders
		// several lux with nothing in front of the sensor. It is also what the backlight follows,
		// so the number reported is the one the panel is acting on.
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

// autoBacklight sets the brightness from the light sensor, when the panel is set to automatic.
//
// It follows a smoothed level rather than the reading itself: at four readings a second a hand
// crossing the sensor would otherwise step the panel and step it back.
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

	// The stored level is the bias here rather than a level to set, so the slider is a preference
	// the curve is read through rather than something automatic overwrites.
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
