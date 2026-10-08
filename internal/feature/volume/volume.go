package volume

import (
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	sharedvolume "github.com/ygelfand/libcountertop/pkg/audio/volume"
	sharedview "github.com/ygelfand/libcountertop/pkg/display/volume"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/feedback"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(20))
}

const Step = sharedvolume.Step

type Change = sharedvolume.Change

type Volume struct {
	entities *sharedvolume.Entities
	Changed  sharedvolume.Changes

	mu      sync.Mutex
	levels  map[config.Stream]int
	numbers map[config.Stream]*esphome.Number

	sounding config.Stream

	card *sharedview.Card
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
		shared.card = sharedview.NewCard(shell.Get())
		shared.build()
	})
	return shared
}

func (v *Volume) Name() string { return "volume" }

func (v *Volume) Entities() []esphome.Entity { return v.entities.Entities() }

func (v *Volume) Restore(c config.Config) {
	v.duck.Set(float32(c.Media.DuckDB))
	for _, s := range config.Streams() {
		v.hold(s, c.Volume.Level(s))
	}
}

func (v *Volume) Level(s config.Stream) int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.levels[s]
}

func (v *Volume) Set(s config.Stream, level int) {
	level = sharedvolume.Clamp(level)
	if v.Level(s) == level {
		return
	}
	v.hold(s, level)

	if showing(s) {
		v.card.Stir()
	} else {
		v.card.Show(s)
	}

	if err := config.Set().Volume().Level(s, level); err != nil {
		slog.Error("saving a volume failed", "stream", s, "err", err)
	}
}

func (v *Volume) Adjust(s config.Stream, steps int) {
	v.Set(s, v.Level(s)+steps*Step)
	feedback.Volume()

	if showing(s) {
		shell.Get().Redraw()
	}
}

func (v *Volume) Target() config.Stream {
	if s, ok := v.card.Selected(); ok {
		return s
	}
	return v.Sounds()
}

func (v *Volume) Sounds() config.Stream {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.sounding
}

func (v *Volume) Sounding(s config.Stream) {
	v.mu.Lock()
	v.sounding = s
	level := v.levels[s]
	v.mu.Unlock()

	speaker.Get().SetVolume(speaker.Gain(level))
}

func (v *Volume) hold(s config.Stream, level int) {
	level = sharedvolume.Clamp(level)

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

func (v *Volume) SetDuckDB(db float64) error { return v.entities.SetDuckDB(db) }

func (v *Volume) build() {
	streams := config.Streams()
	v.entities = sharedvolume.NewEntities(v, streams, config.MediaSection, component.DevicePlayback)
	v.numbers = v.entities.Numbers
	v.duck = v.entities.Duck
}

func (v *Volume) Watch(stream config.Stream, changed func(int)) func() {
	return v.Changed.Watch(stream, changed)
}
