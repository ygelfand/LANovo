package tones

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/audio/tones"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/layout"
)

func init() { component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(15)) }

var get = sync.OnceValue(func() *tones.Store {
	s := tones.New(tones.Options{
		Dir:      layout.ToneDir,
		Rate:     speaker.Rate,
		Channels: speaker.Channels,
	})
	s.Changed.Listen(func(struct{}) { component.Reconnect.Emit(struct{}{}) })
	return s
})

func Get() *tones.Store { return get() }
