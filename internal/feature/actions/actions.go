package actions

import (
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	shared "github.com/ygelfand/libcountertop/pkg/actions"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
)

func init() { component.Register(sharedcomponent.Network, Get) }

type Actions struct{ reg shared.Registry }

var get = sync.OnceValue(func() *Actions {
	a := &Actions{}
	a.reg.Add(call.Get().Action(discovery.Get().Peers, board.Current().CameraWidth > 0))
	component.Subscribed.Listen(func(struct{}) {
		component.Fire.Emit(sharedcomponent.Event{Name: shared.Event, Data: a.reg.Declaration()})
	})
	return a
})

func Get() *Actions { return get() }

func (a *Actions) Name() string { return "actions" }

func (a *Actions) Actions() []*esphome.Action { return a.reg.Actions() }
