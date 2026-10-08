package activity

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/assistant/activity"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
)

func init() { component.Register(sharedcomponent.Device, Get) }

const TurnEvent = "esphome.echolocal_turn"

var get = sync.OnceValue(func() *activity.Log {
	return activity.New(activity.Options{
		Emit: func(fields map[string]string) {
			component.Fire.Emit(sharedcomponent.Event{Name: TurnEvent, Data: fields})
		},
	})
})

func Get() *activity.Log { return get() }
