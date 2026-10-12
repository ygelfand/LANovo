package radio

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/homeassistant/radio"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
)

var get = sync.OnceValue(func() *radio.Radio {
	return radio.New(radio.Options{
		Controller: homeassistant.Get(),
		MAC:        func() string { return wifi.Get().MAC() },
		Settings:   config.RadioSection,
		Redraw:     func() { shell.Get().Redraw() },
	})
})

func Get() *radio.Radio { return get() }
