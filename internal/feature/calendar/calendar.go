package calendar

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/homeassistant/calendar"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/states"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(61))
}

type Calendar struct{ *calendar.Calendar }

func (c Calendar) Restore(cfg config.Config) { c.Publish(cfg.Calendar) }

var get = sync.OnceValue(func() *Calendar {
	c := &Calendar{calendar.New(calendar.Options{
		Controller: homeassistant.Get(),
		States:     states.Get(),
		Settings:   config.CalendarSection,
		Redraw:     func() { shell.Get().Redraw() },
		DeviceID:   component.DeviceScreen,
	})}
	component.Settings.Add(c.Settings())
	return c
})

func Get() *Calendar { return get() }
