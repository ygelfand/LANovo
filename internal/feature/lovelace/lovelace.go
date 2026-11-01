package lovelace

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/homeassistant/lovelaceview"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/chromecast"
	"github.com/ygelfand/LANovo/internal/feature/product"
	"github.com/ygelfand/LANovo/internal/layout"
)

func init() {
	component.Register(sharedcomponent.Network, Get)
}

var Get = sync.OnceValue(func() *lovelaceview.Feature {
	return lovelaceview.Build(
		product.Get(),
		config.LovelaceSection,
		chromecast.Engine(),
		func() string {
			return layout.Slug(config.Get().Device.Name)
		},
	)
})
