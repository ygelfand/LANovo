package homecontrol

import (
	"sync"

	shareddashboard "github.com/ygelfand/libcountertop/pkg/display/dashboard"
	sharedhome "github.com/ygelfand/libcountertop/pkg/homeassistant/homecontrol"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/states"
)

var get = sync.OnceValue(func() *sharedhome.Engine {
	e := sharedhome.New(
		sharedhome.Dependencies{
			Settings: config.HomeSection,
			Client:   homeassistant.Get(),
			States:   states.Get(),
			Shell:    shell.Get(),
			Grid: schema.TesseraGrid{
				Columns: board.Current().Tessera.Columns,
				Rows:    board.Current().Tessera.Rows,
			},
		},
	)
	component.Settings.Add(e.Controls())
	return e
})

func Get() *sharedhome.Engine { return get() }
func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(6))
	dashboard.Tabs().Add(10, func() []shareddashboard.Tab { return Get().DashboardTabs() })
}
