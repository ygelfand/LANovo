package idle

import (
	"sync"
	"time"

	"github.com/ygelfand/libcountertop/pkg/display/idleview"
	"github.com/ygelfand/libcountertop/pkg/display/panel"
	"github.com/ygelfand/libcountertop/pkg/display/videopage"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	"github.com/ygelfand/LANovo/internal/feature/drawer"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	hwtouch "github.com/ygelfand/LANovo/internal/hardware/touch"
)

type Idle struct{ *idleview.Idle }

var (
	once   sync.Once
	shared *Idle
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(35))
}

func Get() *Idle {
	once.Do(func() {
		shared = &Idle{idleview.NewIdle(idleview.Options{
			Settings: config.IdleSection,
			DeviceID: component.DeviceScreen,
			Shell:    shell.Get(),
			View:     newView(),
			Ready: func() bool {
				select {
				case <-clock.Get().Ready():
					return true
				default:
					return false
				}
			},
			Activity: []idleview.Activity{
				func() (time.Duration, bool) { return hwtouch.Get().Since(), true },
				func() (time.Duration, bool) {
					w := videopage.Watched()
					return time.Since(w), !w.IsZero()
				},
				func() (time.Duration, bool) {
					if !config.Get().Presence.Wake {
						return 0, false
					}
					return sensors.Get().Seen()
				},
			},
			Free: func() bool {
				return display.Get().Uncovered(panel.PriorityDashboard) &&
					web.Get().Offering() == ""
			},
		})}
		sensors.Get().Arrived.Listen(func(near bool) {
			if near && config.Get().Presence.Wake {
				shared.Wake()
			}
		})
		drawer.Get().Add(shared.Rail())
	})
	return shared
}

func (i *Idle) Restore(cfg config.Config) { i.Idle.Restore(cfg.Idle) }
