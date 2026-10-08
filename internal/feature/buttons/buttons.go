package buttons

import (
	"log/slog"
	"sync"

	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/buttons"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(10))
}

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

func (b *Buttons) on(e buttons.Event) {
	slog.Debug("button", "which", e.Button, "pressed", e.Pressed)

	if !e.Pressed {
		return
	}

	switch e.Button {
	case buttons.VolumeUp:
		volume.Get().Adjust(volume.Get().Target(), 1)
	case buttons.VolumeDown:
		volume.Get().Adjust(volume.Get().Target(), -1)
	}
}
