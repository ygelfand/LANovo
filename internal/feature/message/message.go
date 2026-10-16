package message

import (
	"sync"

	shared "github.com/ygelfand/libcountertop/pkg/display/message"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/alerts"
)

var get = sync.OnceValue(func() *shared.Messages {
	m := shared.New()
	m.SoundWith(alerts.Get())
	return m
})

func Get() *shared.Messages { return get() }

func init() { component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(40)) }
