// Package privacy is the two physical controls that decide what the device may hear and see.
//
// Both are sliders a person moves, not switches the device can throw, so Home Assistant is told
// what they are rather than offered a way to change them. That is the point of a hardware mute:
// nothing in software can undo it.
package privacy

import (
	"context"
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/feedback"
	"github.com/ygelfand/LANovo/internal/hardware/buttons"
	"github.com/ygelfand/libcountertop/pkg/hook"
)

func init() {
	component.Register(component.Device, Get, component.Order(15))
}

type Privacy struct {
	Changed hook.Hook[Marks]

	micMuted *esphome.BinarySensor
	camera   *esphome.BinarySensor

	mu sync.Mutex
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
	})
	return shared
}

func (p *Privacy) Name() string { return "privacy" }

// Start reads where the sliders already are, rather than waiting to be told they moved.
//
// A slider nobody has touched since boot sends no event, so without this the device comes up
// believing the microphone is live because it has not seen it muted — which is the wrong way for
// a mute to fail. The lines belong to the hardware phase and are open by the time this runs.
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

// MicMuted reports whether the microphone slider is over, which is what anything about to record
// has to ask.
func (p *Privacy) MicMuted() bool { return p.micMuted.Get() }

// CameraCovered reports whether the shutter is closed.
func (p *Privacy) CameraCovered() bool { return p.camera.Get() }

// on follows the sliders. The driver reports a slider engaged as pressed, and for the camera that
// already means the shutter is closed.
func (p *Privacy) on(e buttons.Event) {
	switch e.Button {
	case buttons.MicMute:
		p.micMuted.Set(e.Pressed)
		slog.Info("microphone", "muted", e.Pressed)

		// The slider is a physical cut: the tone says the device noticed, the corner mark says so
		// afterwards.
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

// Watch calls changed when a physical privacy control changes.
func (p *Privacy) Watch(changed func()) func() {
	return p.Changed.Listen(func(Marks) { changed() })
}
