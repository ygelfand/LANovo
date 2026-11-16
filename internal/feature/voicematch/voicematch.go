package voicematch

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/assistant/voicematch"
	ha "github.com/ygelfand/libcountertop/pkg/homeassistant"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/layout"
)

func init() { component.Register(sharedcomponent.Device, Get) }

var get = sync.OnceValue(func() *voicematch.Service {
	s := voicematch.NewService(voicematch.Options{
		Speech:   mic.Get().History,
		ModelDir: layout.VoiceprintDir,
		Profiles: layout.VoicesPath,
		Outbox:   layout.VoicesOutbox,
		Home:     homeassistant.Get(),
	})
	homeassistant.Get().Changed.Listen(func(a ha.Access) {
		if a == ha.Allowed {
			s.Resync()
		}
	})
	return s
})

func Get() *voicematch.Service { return get() }
