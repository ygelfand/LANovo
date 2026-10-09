package poster

import (
	"sync"

	sharedposter "github.com/ygelfand/libcountertop/pkg/display/poster"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	hwtouch "github.com/ygelfand/LANovo/internal/hardware/touch"
)

type componentPoster struct{ *sharedposter.Poster }

func (p *componentPoster) Restore(c config.Config) { p.Poster.Restore(c.Poster) }

var get = sync.OnceValue(func() *sharedposter.Poster {
	p := sharedposter.New(
		sharedposter.Dependencies{
			Settings: config.PosterSection,
			Touch:    hwtouch.Get(),
			Ambient:  sensors.Get().Ambient,
			DeviceID: component.DeviceScreen,
		},
	)
	component.Settings.Add(p.Settings())
	return p
})

func Get() *sharedposter.Poster { return get() }
func init() {
	component.Register(
		sharedcomponent.Device,
		func() *componentPoster { return &componentPoster{Get()} },
		sharedcomponent.Order(25),
	)
}
