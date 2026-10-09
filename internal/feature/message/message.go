package message

import (
	"image"
	"sync"

	shared "github.com/ygelfand/libcountertop/pkg/display/message"
	"github.com/ygelfand/libcountertop/pkg/input/touch"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/alerts"
	hwtouch "github.com/ygelfand/LANovo/internal/hardware/touch"
)

var get = sync.OnceValue(func() *shared.Messages {
	m := shared.New()
	hwtouch.Get().Contacts.Listen(func(c touch.Contact) {
		if c.Phase == touch.Down {
			m.Dismiss(c.ID, image.Pt(c.X, c.Y))
		}
	})
	m.SoundWith(alerts.Get())
	return m
})

func Get() *shared.Messages { return get() }

func init() { component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(40)) }
