package message

import (
	"sync"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/hardware/touch"
	shared "github.com/ygelfand/libcountertop/pkg/display/message"
)

var get = sync.OnceValue(func() *shared.Messages {
	m := shared.New()
	touch.Get().Contacts.Listen(func(c touch.Contact) {
		if c.Phase == touch.Down {
			m.Dismiss(c.ID)
		}
	})
	return m
})

func Get() *shared.Messages { return get() }
func init()                 { component.Register(component.Device, Get, component.Order(40)) }
