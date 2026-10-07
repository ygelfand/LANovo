// Package states is what Home Assistant knows, for the parts of the device that want to show it.
//
// The device cannot go and read a state. Home Assistant asks, once per connection, what this
// device wants to be told about; the device answers with a list; and from then on every change to
// one of those arrives unprompted. So a follower is registered before anything connects, and the
// list is offered again on every connection.
//
// This is the same route ESPHome's own `homeassistant` platforms take, which is why a value shows
// up without anything being configured on the server: the integration honors the list a device
// sends it.
package states

import (
	"github.com/ygelfand/LANovo/internal/component"
	sharedstates "github.com/ygelfand/libcountertop/pkg/homeassistant/states"
)

type States = sharedstates.States
type Value = sharedstates.Value

var shared = sharedstates.New()

func Get() *States { return shared }
func init() {
	component.Register(component.Device, Get, component.Order(5))
}
