package radar

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/radar"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/layout"
)

var get = sync.OnceValue(func() *radar.Radar {
	return radar.New(radar.Options{
		Home:     homeassistant.Get().Home,
		Agent:    "(LANovo " + layout.Version + ", github.com/ygelfand/LANovo)",
		Settings: config.WeatherSection,
		Redraw:   func() { shell.Get().Redraw() },
		CacheDir: layout.TileDir,
	})
})

func Get() *radar.Radar { return get() }
