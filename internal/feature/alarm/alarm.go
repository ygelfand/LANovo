package alarm

import (
	"sync"
	"time"

	"github.com/ygelfand/libcountertop/pkg/assistant/alarm"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/remember"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/alerts"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	featureremember "github.com/ygelfand/LANovo/internal/feature/remember"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(32))
}

var get = sync.OnceValue(func() *alarm.Alarms {
	a := alarm.New(alarm.Options{
		Saved:   remember.NewSlot[[]alarm.Alarm](featureremember.Get(), "alarms"),
		Snoozes: remember.NewSlot[map[string]time.Time](featureremember.Get(), "alarm_snoozes"),
		Chime:   alerts.Get().RingAlarm,
		Duck: func() func() {
			sound := speaker.Sound()
			sound.Backgrounds().Duck(true)
			return func() { sound.Backgrounds().Duck(false) }
		},
		RingFor: alerts.Get().RingFor,
		Trusted: clock.Get().Set,
	})
	a.Told.Listen(component.Fire.Emit)
	return a
})

func Get() *alarm.Alarms { return get() }
