package homecontrol

import (
	"context"
	"sync"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/states"
	sharedhome "github.com/ygelfand/libcountertop/pkg/homeassistant/homecontrol"
)

type Selection = sharedhome.Selection
type Picker = sharedhome.Picker
type Board = sharedhome.Board
type Home = sharedhome.Home
type Tab = sharedhome.Tab
type Snapshot = sharedhome.Snapshot
type LabelRow = sharedhome.LabelRow
type EntityRow = sharedhome.EntityRow
type AreaRow = sharedhome.AreaRow
type Tile = sharedhome.Tile
type AreaTiles = sharedhome.AreaTiles

const (
	TabLabels = sharedhome.TabLabels
	TabManual = sharedhome.TabManual
	TabKind   = sharedhome.TabKind
)

var Includes = sharedhome.Includes
var Summary = sharedhome.Summary
var once sync.Once
var shared *sharedhome.Engine

func engine() *sharedhome.Engine {
	once.Do(func() {
		shared = sharedhome.New(
			sharedhome.Options{
				Read:    func() config.Home { return config.Get().Home },
				Pick:    func(k string, v config.HomePick) error { return config.Set().Home().Pick(k, v) },
				Group:   func(k string, v int) error { return config.Set().Home().Group(k, v) },
				Control: func(k string, v bool) error { return config.Set().Home().Control(k, v) },
				Enabled: func(v bool) error { return config.Set().Home().Enabled(v) },
				Combine: func(v bool) error { return config.Set().Home().Combine(v) },
				Enable:  homeassistant.Get().Enable,
				Probe:   homeassistant.Get().Probe,
				Fetch: func(ctx context.Context, f homeassistant.Filter) ([]homeassistant.Entity, []homeassistant.Label, error) {
					a := homeassistant.Get()
					entities, err := a.Entities(ctx, f)
					if err != nil {
						return nil, nil, err
					}
					labels, err := a.Labels(ctx)
					return entities, labels, err
				},
				Follow: states.Get().Follow,
				Call:   homeassistant.Get().Call,
				Shell:  shell.Get(),
				Changed: func(f func(bool)) func() {
					return homeassistant.Get().Changed.Listen(
						func(a homeassistant.Access) { f(a == homeassistant.Allowed) },
					)
				},
			},
		)
	})
	return shared
}
func init() {
	component.Register(component.Device, engine, component.Order(6))
	dashboard.AddTabs(10, func() []dashboard.Tab {
		out := []dashboard.Tab{}
		for _, s := range Dash().Tabs() {
			out = append(out, dashboard.Tab{Kind: TabKind, Key: s.Key, Name: s.Name()})
		}
		return out
	})
}
func Selections() []Selection               { return engine().Selections() }
func NewPicker(s Selection) *Picker         { return engine().NewPicker(s) }
func SelectionPage(s Selection) *shell.Page { return engine().SelectionPage(s) }
func Page() *Home                           { return engine().Page() }
func Dash() *Board                          { return engine().Dash() }
func Enabled() bool                         { return engine().Enabled() }
func SetEnabled(v bool)                     { engine().SetEnabled(v) }
func Combined() bool                        { return engine().Combined() }
func SetCombined(v bool)                    { engine().SetCombined(v) }
func Refresh()                              { engine().Refresh() }

func Engine() *sharedhome.Engine { return engine() }
