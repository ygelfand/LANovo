package todo

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/homeassistant/todo"
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

type Todo struct{ *todo.Todo }

func (t Todo) Restore(cfg config.Config) { t.Publish(cfg.Todo) }

var get = sync.OnceValue(func() *Todo {
	t := &Todo{todo.New(todo.Options{
		Controller: homeassistant.Get(),
		States:     states.Get(),
		Settings:   config.TodoSection,
		Redraw:     func() { shell.Get().Redraw() },
		DeviceID:   component.DeviceScreen,
	})}
	component.Settings.Add(t.Settings())
	return t
})

func Get() *Todo { return get() }
