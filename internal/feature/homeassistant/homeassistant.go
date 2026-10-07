package homeassistant

import (
	"sync"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	sharedha "github.com/ygelfand/libcountertop/pkg/homeassistant"
)

var get = sync.OnceValue(func() *sharedha.Controller {
	return sharedha.New(sharedha.Settings{Home: config.HomeSection, API: config.APISection})
})

func Get() *sharedha.Controller { return get() }
func init()                     { component.Register(component.Device, Get, component.Order(5)) }
