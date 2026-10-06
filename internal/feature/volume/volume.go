// Package volume is how loud the device is, split by what is making the sound.
//
// The card has one gain, and the streams are how loud each kind of sound should be through it, so
// the gain follows whatever is sounding. Set on the card rather than multiplied into the samples,
// so moving a slider reaches audio already queued.
package volume

import (
	"github.com/ygelfand/libcountertop/pkg/audio/ducking"
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/feedback"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/libcountertop/pkg/hook"
)

func init() {
	component.Register(component.Device, Get, component.Order(20))
}

// Step is how far one press of a volume button moves the level.
const Step = 5

// Change is a level that just moved.
type Change struct {
	Stream config.Stream
	Level  int
}

type Volume struct {
	// Changed carries every change, however it was made.
	Changed hook.Hook[Change]

	mu      sync.Mutex
	levels  map[config.Stream]int
	numbers map[config.Stream]*esphome.Number

	// sounding is the stream the card is carrying, which is the one its gain follows.
	sounding config.Stream

	card card
	duck *esphome.Number
}

var (
	once   sync.Once
	shared *Volume
)

func Get() *Volume {
	once.Do(func() {
		shared = &Volume{
			levels:   map[config.Stream]int{},
			numbers:  map[config.Stream]*esphome.Number{},
			sounding: config.StreamMedia,
		}
		shared.build()
	})
	return shared
}

func (v *Volume) Name() string { return "volume" }

func (v *Volume) Entities() []esphome.Entity {
	out := make([]esphome.Entity, 0, len(v.numbers))
	for _, s := range config.Streams() {
		out = append(out, v.numbers[s])
	}
	return append(out, v.duck)
}

// Restore puts the levels back where they were left.
func (v *Volume) Restore(c config.Config) {
	v.duck.Set(float32(c.Media.DuckDB))
	for _, s := range config.Streams() {
		v.hold(s, c.Volume.Level(s))
	}
}

// Level is how loud one kind of sound is.
func (v *Volume) Level(s config.Stream) int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.levels[s]
}

// TODO: a mute is a level of 0 today; keep the level it replaced to allow unmute.
//
// Set changes a level and remembers it.
func (v *Volume) Set(s config.Stream, level int) {
	level = clamp(level)
	if v.Level(s) == level {
		return
	}
	v.hold(s, level)

	// The card says what the level is now, which is worth saying over a clock, over the dock and
	// over any other screen — but not over the one showing the levels, which it outranks: it would
	// cover the slider being dragged with a picture of the slider being dragged.
	//
	// The card counts as showing once it is up, so this is also the path every change after the
	// first takes.
	if showing(s) {
		v.card.stir()
	} else {
		v.card.show(Change{Stream: s, Level: level})
	}

	if err := config.Set().Volume().Level(s, level); err != nil {
		slog.Error("saving a volume failed", "stream", s, "err", err)
	}
}

// Adjust moves a level by a number of steps, which is what a button press is.
//
// It beeps and Set does not: a press wants an answer at the new level, and a slider dragged in Home
// Assistant would chirp at every step on the way.
//
// It also repaints an open screen, which Set cannot: Set is what a finger dragging a slider calls,
// and that repaints the one row it moved rather than the page.
func (v *Volume) Adjust(s config.Stream, steps int) {
	v.Set(s, v.Level(s)+steps*Step)
	feedback.Volume()

	// A press while the levels are on screen moves one under the reader, and the card that would
	// otherwise have said so is suppressed there.
	if showing(s) {
		shell.Get().Redraw()
	}
}

// Target is the stream a volume button moves.
//
// The card while it is up, because the circle on it says which level is being changed and a button
// that moved a different one would make that a lie. Otherwise whatever is sounding, which is the
// only sensible answer when there is nothing on screen to say.
//
// So a press raises the card on the stream that is playing, another press moves the same one, and
// picking a different stream on the card moves the buttons to it — which is the whole of what the
// card being a selection buys.
func (v *Volume) Target() config.Stream {
	if s, ok := (&v.card).selected(); ok {
		return s
	}
	return v.Sounds()
}

// Sounds is the stream the card's gain is following.
func (v *Volume) Sounds() config.Stream {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.sounding
}

// Sounding says what the card is carrying now, so its gain follows that stream.
func (v *Volume) Sounding(s config.Stream) {
	v.mu.Lock()
	v.sounding = s
	level := v.levels[s]
	v.mu.Unlock()

	speaker.Get().SetVolume(speaker.Gain(level))
}

// hold records a level and tells everyone, without writing it back to the file it may have come
// from.
func (v *Volume) hold(s config.Stream, level int) {
	level = clamp(level)

	v.mu.Lock()
	v.levels[s] = level
	number := v.numbers[s]
	live := v.sounding == s
	v.mu.Unlock()

	if live {
		speaker.Get().SetVolume(speaker.Gain(level))
	}

	number.Set(float32(level))
	v.Changed.Emit(Change{Stream: s, Level: level})
}

// SetDuckDB saves the media ducking depth and publishes its effective value.
func (v *Volume) SetDuckDB(db float64) error {
	if err := config.Set().Media().DuckDB(db); err != nil {
		return err
	}
	v.duck.Set(float32(db))
	return nil
}

func (v *Volume) build() {
	v.duck = &esphome.Number{Base: esphome.Base{ObjectID: "media_duck_level", Name: "Music ducking", Icon: "mdi:volume-medium", Category: esphome.CategoryConfig, DeviceID: component.DevicePlayback}, Min: ducking.MinimumDB, Max: ducking.MaximumDB, Step: 1, Unit: "dB", Mode: esphome.NumberBox}
	v.duck.OnCommand = func(db float32) {
		if err := v.SetDuckDB(float64(db)); err != nil {
			slog.Error("saving the ducking level failed", "err", err)
		}
	}

	for _, s := range config.Streams() {
		stream := s

		n := &esphome.Number{
			Base: esphome.Base{
				ObjectID: "volume_" + string(stream),
				Name:     stream.Label() + " volume",
				Icon:     "mdi:volume-high",
				Category: esphome.CategoryConfig,
				DeviceID: component.DevicePlayback,
			},
			Min: 0, Max: 100, Step: 1, Unit: "%",
			Mode: esphome.NumberSlider,
		}
		n.OnCommand = func(level float32) { shared.Set(stream, int(level)) }

		v.numbers[stream] = n
	}
}

func clamp(level int) int {
	switch {
	case level < 0:
		return 0
	case level > 100:
		return 100
	}
	return level
}
