package radio

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/homeassistant/radio"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/product"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
)

var get = sync.OnceValue(func() *radio.Radio {
	return radio.Build(
		product.Get(),
		config.RadioSection,
		func() string { return wifi.Get().MAC() },
	)
})

func Get() *radio.Radio { return get() }
