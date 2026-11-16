package voicematch

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/assistant/voicematch"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/layout"
)

func init() { component.Register(sharedcomponent.Device, Get) }

var get = sync.OnceValue(func() *voicematch.Service {
	return voicematch.NewService(voicematch.Options{
		Speech:   mic.Get().History,
		ModelDir: layout.VoiceprintDir,
		Profiles: layout.VoicesPath,
	})
})

func Get() *voicematch.Service { return get() }
