package alerts

import (
	"sync"

	shared "github.com/ygelfand/libcountertop/pkg/audio/alerts"
	"github.com/ygelfand/libcountertop/pkg/audio/tone"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/tones"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func init() { component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(30)) }

var get = sync.OnceValue(func() *shared.Alerts {
	a := shared.New(
		config.AlertsSection,
		component.DevicePlayback,
		tones.Get(),
		func(s tone.Sound) {
			speaker.Sound().Sound(schema.StreamAlerts, shared.Level, s)
		},
	)
	component.Settings.Add(a.Controls)
	tones.Get().Changed.Listen(func(struct{}) { a.Refresh() })
	return a
})

func Get() *shared.Alerts { return get() }
