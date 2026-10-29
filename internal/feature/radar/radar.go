package radar

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/radar"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/product"
)

var get = sync.OnceValue(
	func() *radar.Radar { return radar.Build(product.Get(), config.WeatherSection) },
)

func Get() *radar.Radar { return get() }
