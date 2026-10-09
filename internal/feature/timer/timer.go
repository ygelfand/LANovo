package timer

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/assistant/timer"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/alerts"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(31))
}

var get = sync.OnceValue(func() *timer.Timers {
	return timer.New(timer.Options{
		Alarm: func() func() {
			sound := speaker.Sound()
			sound.Backgrounds().Duck(true)
			return func() { sound.Backgrounds().Duck(false) }
		},
		Chime:   alerts.Get().RingTimer,
		RingFor: alerts.Get().RingFor,
	})
})

func Get() *timer.Timers { return get() }
