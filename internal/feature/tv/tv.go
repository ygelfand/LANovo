package tv

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/homeassistant/yttv"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/chromecast"
	"github.com/ygelfand/LANovo/internal/feature/product"
)

func init() { component.Register(sharedcomponent.Network, Get) }

var get = sync.OnceValue(func() *yttv.Guide {
	return yttv.Build(product.Get(), config.TVSection, chromecast.ID)
})

func Get() *yttv.Guide { return get() }
