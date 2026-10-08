package states

import (
	sharedstates "github.com/ygelfand/libcountertop/pkg/homeassistant/states"

	"github.com/ygelfand/LANovo/internal/component"
)

type States = sharedstates.States
type Value = sharedstates.Value

var shared = sharedstates.New()

func Get() *States { return shared }
func init() {
	component.Register(component.Device, Get, component.Order(5))
}
