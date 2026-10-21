package actions

import (
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	shared "github.com/ygelfand/libcountertop/pkg/actions"
	sharedha "github.com/ygelfand/libcountertop/pkg/homeassistant"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/voice"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/broadcast"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/states"
)

func init() { component.Register(sharedcomponent.Network, Get) }

type Actions struct{ reg shared.Registry }

var get = sync.OnceValue(func() *Actions {
	a := &Actions{}
	a.reg.Add(call.Get().Action(discovery.Get().Peers, board.Current().CameraWidth > 0))
	a.reg.Add(broadcast.Get().Action())
	component.Subscribed.Listen(func(struct{}) { a.Declare() })
	states.Get().Follow(sharedha.VersionEntity, "").Reported.Listen(func(string) { a.Declare() })
	return a
})

func Get() *Actions { return get() }

func (a *Actions) Names() []string { return a.reg.Names() }

func (a *Actions) Declare() {
	voiced := config.Get().Voice.Action
	component.Fire.Emit(
		sharedcomponent.Event{
			Name: shared.Event,
			Data: a.reg.Declaration(voiced, voice.ActionSwitch),
		},
	)
}

func (a *Actions) Name() string { return "actions" }

func (a *Actions) Actions() []*esphome.Action { return a.reg.Actions() }
