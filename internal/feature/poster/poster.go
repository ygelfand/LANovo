// Package poster registers the shared photo backdrop with the device's physical sensors.
package poster

import (
	"sync"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/hardware/touch"
	sharedposter "github.com/ygelfand/libcountertop/pkg/display/poster"
)

type componentPoster struct{ *sharedposter.Poster }

func (p *componentPoster) Restore(c config.Config) { p.Poster.Restore(c.Poster) }

var get = sync.OnceValue(func() *sharedposter.Poster {
	return sharedposter.New(sharedposter.Dependencies{Settings: config.PosterSection, Touch: touch.Get(), Ambient: sensors.Get().Ambient, DeviceID: component.DeviceScreen})
})

func Get() *sharedposter.Poster { return get() }
func init() {
	component.Register(component.Device, func() *componentPoster { return &componentPoster{Get()} }, component.Order(25))
}
