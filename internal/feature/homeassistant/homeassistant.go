package homeassistant

import (
	"sync"

	sharedha "github.com/ygelfand/libcountertop/pkg/homeassistant"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
)

var get = sync.OnceValue(func() *sharedha.Controller {
	return sharedha.New(sharedha.Settings{Home: config.HomeSection, API: config.APISection})
})

func Get() *sharedha.Controller { return get() }

func init() { component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(5)) }
