package assistant

import (
	"sync"

	sharedscreen "github.com/ygelfand/libcountertop/pkg/display/assistant"
	"github.com/ygelfand/libcountertop/pkg/input/touch"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/feature/voice"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/gpu"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	hwtouch "github.com/ygelfand/LANovo/internal/hardware/touch"
)

func init() { component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(45)) }

var get = sync.OnceValue(func() *sharedscreen.Assistant {
	a := sharedscreen.New(sharedscreen.Dependencies{Microphone: mic.Get(), Controller: voice.Get()})
	voice.Shown.Listen(a.Show)
	hwtouch.Get().Contacts.Listen(func(c touch.Contact) {
		if c.Phase == touch.Down {
			a.Cancel(c.ID)
		}
	})
	return a
})

func Get() *sharedscreen.Assistant { return get() }

var Stage = sync.OnceValue(func() *sharedscreen.Stage {
	return sharedscreen.NewStage(sharedscreen.StageDependencies{
		Screen:  Get(),
		GPU:     gpu.Get(),
		Display: display.Get(),
		Visuals: visuals.Get(),
		Wake:    config.WakeSection,
	})
})
