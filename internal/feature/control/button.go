package control

import (
	"github.com/ygelfand/LANovo/internal/hardware/buttons"
	"github.com/ygelfand/LANovo/internal/hardware/touch"
	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"
)

func (c *Control) inputEngine() *harness.Input {
	c.inputOnce.Do(func() { c.input = newInput() })
	return c.input
}
func newInput() *harness.Input {
	options := harness.InputOptions{}
	options.Buttons = []string{"up", "down"}
	for _, b := range buttons.All() {
		options.Buttons = append(options.Buttons, string(b))
	}
	options.Press = func(name string) error {
		b := buttons.Button(name)
		if name == "up" {
			b = buttons.VolumeUp
		}
		if name == "down" {
			b = buttons.VolumeDown
		}
		buttons.Get().Deliver(buttons.Event{Button: b, Pressed: true})
		buttons.Get().Deliver(buttons.Event{Button: b, Pressed: false})
		return nil
	}
	options.Deliver = func(c harness.Contact) {
		touch.Get().Deliver(touch.Contact{ID: c.ID, X: c.X, Y: c.Y, Phase: touch.Phase(c.Phase), At: c.At})
	}
	return harness.NewInput(options)
}
