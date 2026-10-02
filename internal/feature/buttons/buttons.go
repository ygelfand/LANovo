// Package buttons is what the physical controls do.
//
// The driver reads the lines and says what changed; this decides what it means. They are separate
// packages because a button is a button whether the device acts on it, tells Home Assistant, or
// both.
package buttons

import (
	"log/slog"
	"sync"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/buttons"
)

func init() {
	component.Register(component.Device, Get, component.Order(10))
}

// Buttons acts on what the driver reports.
type Buttons struct{}

var (
	once   sync.Once
	shared *Buttons
)

func Get() *Buttons {
	once.Do(func() {
		shared = &Buttons{}
		buttons.Get().Events.Listen(shared.on)
	})
	return shared
}

func (b *Buttons) Name() string { return "button actions" }

// on runs on the driver's reader goroutine, so anything slow goes on a goroutine of its own.
func (b *Buttons) on(e buttons.Event) {
	slog.Info("button", "which", e.Button, "pressed", e.Pressed)

	// Acted on when the button goes down, not when it comes back up.
	if !e.Pressed {
		return
	}

	// Whichever stream the card is showing, or what is sounding when it is not up. Media was
	// hardcoded here, which meant a press while the card showed the alerts level moved the media
	// one and the bar on screen was describing something else.
	switch e.Button {
	case buttons.VolumeUp:
		volume.Get().Adjust(volume.Get().Target(), 1)
	case buttons.VolumeDown:
		volume.Get().Adjust(volume.Get().Target(), -1)
	}
}
