package homecontrol

import (
	"sync"

	shareddashboard "github.com/ygelfand/libcountertop/pkg/display/dashboard"
	sharedhome "github.com/ygelfand/libcountertop/pkg/homeassistant/homecontrol"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

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
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(6))
	dashboard.Tabs().Add(10, func() []shareddashboard.Tab { return Get().DashboardTabs() })
}
