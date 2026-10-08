package message

import (
	"sync"

	shared "github.com/ygelfand/libcountertop/pkg/display/message"
	"github.com/ygelfand/libcountertop/pkg/input/touch"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	hwtouch "github.com/ygelfand/LANovo/internal/hardware/touch"
)

var get = sync.OnceValue(func() *shared.Messages {
	m := shared.New()
	hwtouch.Get().Contacts.Listen(func(c touch.Contact) {
		if c.Phase == touch.Down {
			m.Dismiss(c.ID)
		}
	})
	return m
})

func Get() *shared.Messages { return get() }

func init() { component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(40)) }
