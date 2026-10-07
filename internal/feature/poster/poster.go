// Package poster puts a picture from Immich behind the dashboard.
package poster

import (
	"sync"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/hardware/touch"
	sharedposter "github.com/ygelfand/libcountertop/pkg/display/poster"
)

type Poster struct{ *sharedposter.Poster }

func (p *Poster) Restore(c config.Config) { p.Poster.Restore(c.Poster) }

var once sync.Once
var shared *Poster

func Get() *Poster {
	once.Do(func() {
		shared = &Poster{
			sharedposter.New(
				sharedposter.Options{
					Read:       func() config.Poster { return config.Get().Poster },
					Enabled:    func(v bool) error { return config.Set().Poster().Enabled(v) },
					Every:      func(v config.PosterEvery) error { return config.Set().Poster().Every(v) },
					Server:     func(v string) error { return config.Set().Poster().Server(v) },
					Key:        func(v string) error { return config.Set().Poster().Key(v) },
					Albums:     func(v string) error { return config.Set().Poster().Albums(v) },
					Tags:       func(v string) error { return config.Set().Poster().Tags(v) },
					Last:       func(v string) error { return config.Set().Poster().Last(v) },
					SinceTouch: touch.Get().Since,
					Ambient:    sensors.Get().Ambient,
					DeviceID:   component.DeviceScreen,
				},
			),
		}
	})
	return shared
}
func Dark() bool { return Get().Poster.Dark() }
func init()      { component.Register(component.Device, Get, component.Order(25)) }
