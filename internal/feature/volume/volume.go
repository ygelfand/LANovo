package volume

import (
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

const (
	Step      = sharedvolume.Step
	MainSteps = 100 / Step
)

type Change = sharedvolume.Change

type Volume struct {
	*sharedvolume.Volume

	card *sharedview.Card
	duck *sharedvolume.Ducking
}

var (
	once   sync.Once
	shared *Volume
)

func Get() *Volume {
	once.Do(func() {
		shared = &Volume{
			Volume: sharedvolume.NewVolume(sharedvolume.Options{
				Mixer: sharedvolume.MixerOptions{
					Output:  speaker.Get(),
					Streams: config.Streams(),
					Save: func(s config.Stream, level int) error {
						return config.Set().Volume().Level(s, level)
					},
					Device: component.DevicePlayback,
				},
			}),
			card: sharedview.NewCard(shell.Get()),
			duck: sharedvolume.NewDucking(config.MediaSection, component.DevicePlayback),
		}
		component.Settings.Add(shared.duck)
	})
	return shared
}

func (v *Volume) Name() string { return "volume" }

func (v *Volume) Entities() []esphome.Entity {
	return append(v.Volume.Entities(), v.duck.Entities()...)
}

func (v *Volume) Restore(c config.Config) {
	v.duck.PublishFrom(c.Media)
	v.Volume.Restore(c.Volume.Level)
}

func (v *Volume) Set(s config.Stream, level int) {
	if v.Level(s) == sharedvolume.Clamp(level) {
		return
	}
	v.Volume.Set(s, level)

	if showing(s) {
		v.card.Stir()
	} else {
		v.card.Show(s)
	}
}

func (v *Volume) Adjust(s config.Stream, steps int) {
	v.Set(s, v.Level(s)+steps*Step)
	feedback.Preview(s)

	if showing(s) {
		shell.Get().Redraw()
	}
}

func (v *Volume) Target() config.Stream {
	return sharedvolume.Target(sharedvolume.Aim{
		Mode:   config.Get().Media.Buttons,
		Picked: v.card.Selected,
		Screen: func() any { return shell.Get().Top() },
		Heard:  func(s config.Stream) bool { return speaker.Get().Heard(s, sharedvolume.Recent) },
	})
}
