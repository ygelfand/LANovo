package timer

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/assistant/timer"
	"github.com/ygelfand/libcountertop/pkg/audio/tone"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(31))
}

const alarmLevel = 0.6

var get = sync.OnceValue(func() *timer.Timers {
	return timer.New(timer.Options{
		Alarm: func() func() {
			sound := speaker.Sound()
			sound.Backgrounds().Duck(true)
			return func() { sound.Backgrounds().Duck(false) }
		},
		Chime: func() {
			speaker.Sound().
				Interject(func(p *speaker.Speaker) { p.Chime(schema.StreamAlerts, alarmLevel, tone.ToneTimer...) })
		},
	})
})

func Get() *timer.Timers { return get() }
