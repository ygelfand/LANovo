package homecontrol

import (
	"sync"

	sharedhome "github.com/ygelfand/libcountertop/pkg/homeassistant/homecontrol"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/states"
)

var get = sync.OnceValue(func() *sharedhome.Engine {
	return sharedhome.New(
		sharedhome.Dependencies{
			Settings: config.HomeSection,
			Client:   homeassistant.Get(),
			States:   states.Get(),
			Shell:    shell.Get(),
		},
	)
})

func Get() *sharedhome.Engine { return get() }
func init() {
	component.Register(component.Device, Get, component.Order(6))
	dashboard.AddTabs(10, func() []dashboard.Tab {
		var out []dashboard.Tab
		for _, tab := range Get().Dash().Tabs() {
			out = append(
				out,
				dashboard.Tab{Kind: sharedhome.TabKind, Key: tab.Key, Name: tab.Name()},
			)
		}
		return out
	})
}
