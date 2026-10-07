package activity

import (
	"github.com/ygelfand/LANovo/internal/component"
	sharedactivity "github.com/ygelfand/libcountertop/pkg/assistant/activity"
	"sync"
)

func init() { component.Register(component.Device, Get) }

type Log = sharedactivity.Log

var get = sync.OnceValue(func() *Log {
	return sharedactivity.New(sharedactivity.Options{
		Emit: func(fields map[string]string) { component.Fire.Emit(component.Event{Name: TurnEvent, Data: fields}) },
	})
})

func Get() *Log { return get() }
