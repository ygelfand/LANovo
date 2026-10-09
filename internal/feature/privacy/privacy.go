package privacy

import (
	"context"
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/hook"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/feedback"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/hardware/buttons"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(15))
}

type Privacy struct {
	Changed hook.Hook[Marks]

	micMuted *esphome.BinarySensor
	camera   *esphome.BinarySensor
}

var (
	once   sync.Once
	shared *Privacy
)

func Get() *Privacy {
	once.Do(func() {
		shared = &Privacy{}
		shared.build()

		buttons.Get().Events.Listen(shared.on)
		screen.Get().Restyled.Listen(func(schema.Screen) { shared.show() })
	})
	return shared
}

func (p *Privacy) Name() string { return "privacy" }

func (p *Privacy) Start(context.Context) error {
	for _, at := range []struct {
		button buttons.Button
		sensor *esphome.BinarySensor
		what   string
	}{
		{buttons.MicMute, p.micMuted, "microphone"},
		{buttons.CameraCover, p.camera, "camera"},
	} {
		on, ok := buttons.Get().State(at.button)
		if !ok {
			slog.Warn("cannot tell where the slider is", "control", at.button)
			continue
		}

		at.sensor.Set(on)
		slog.Info(at.what, "engaged", on)
	}

	p.show()
	return nil
}

func (p *Privacy) Entities() []esphome.Entity {
	return []esphome.Entity{p.micMuted, p.camera}
}

func (p *Privacy) MicMuted() bool { return p.micMuted.Get() }

func (p *Privacy) CameraCovered() bool { return p.camera.Get() }

func (p *Privacy) on(e buttons.Event) {
	switch e.Button {
	case buttons.MicMute:
		p.micMuted.Set(e.Pressed)
		slog.Info("microphone", "muted", e.Pressed)

		if e.Pressed {
			feedback.Muted()
		} else {
			feedback.Unmuted()
		}
	case buttons.CameraCover:
		p.camera.Set(e.Pressed)
		slog.Info("camera", "covered", e.Pressed)
	}

	p.show()
}

func (p *Privacy) build() {
	p.micMuted = &esphome.BinarySensor{
		Base: esphome.Base{
			ObjectID: "microphone_muted",
			Name:     "Microphone muted",
			Icon:     "mdi:microphone-off",
		},
	}

	p.camera = &esphome.BinarySensor{
		Base: esphome.Base{
			ObjectID: "camera_covered",
			Name:     "Camera covered",
			Icon:     "mdi:camera-off",
		},
	}
}

func (p *Privacy) Watch(changed func()) func() {
	return p.Changed.Listen(func(Marks) { changed() })
}
