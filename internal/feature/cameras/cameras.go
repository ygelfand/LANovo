package cameras

import (
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/display/cameraview"
	sharedshell "github.com/ygelfand/libcountertop/pkg/display/shell"
	"github.com/ygelfand/libcountertop/pkg/fetch"
	sharedha "github.com/ygelfand/libcountertop/pkg/homeassistant"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

const fetchWait = 15 * time.Second

func init() {
	component.Register(sharedcomponent.Network, Site)
	component.Register(sharedcomponent.Device, func() *Cameras { return &Cameras{Get()} })
}

var Site = sync.OnceValue(func() *sharedha.Site {
	return &sharedha.Site{Override: func() string { return config.Get().Cameras.URL }}
})

var Get = sync.OnceValue(func() *cameraview.Cameras {
	c := cameraview.New(cameraview.Options{
		Shell:    shell.Get(),
		Source:   homeassistant.Get(),
		Streams:  homeassistant.Get(),
		Speaker:  sound{},
		Display:  display.Get(),
		Site:     Site(),
		Settings: config.CamerasSection,
		Client:   fetch.Client(fetchWait),
		Picker:   picker,
	})
	component.Settings.Add(c.Controls())
	return c
})

type Cameras struct{ *cameraview.Cameras }

type sound struct{}

func (sound) Play(pcm []int16) { speaker.Get().PlayStream(config.StreamMedia, pcm) }

func (sound) Queued() int { return speaker.Get().Queued() }

func (c *Cameras) Name() string { return "cameras" }

func (c *Cameras) Entities() []esphome.Entity { return c.Controls().Entities() }

func (c *Cameras) Restore(cfg config.Config) { c.Controls().PublishFrom(cfg.Cameras) }

func picker() sharedshell.View {
	e := homecontrol.Get()
	return e.NewPicker(e.Choice(
		"cameras",
		sharedha.Filter{Domains: []string{"camera"}},
		func() schema.HomePick { return config.Get().Cameras.Pick },
		func(p schema.HomePick) error {
			return config.CamerasSection.Update(func(c *schema.Cameras) { c.Pick = p })
		},
	))
}
