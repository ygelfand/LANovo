package web

import (
	"github.com/ygelfand/libcountertop/pkg/runtime/status"
	"github.com/ygelfand/libcountertop/pkg/system/metrics"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/layout"
)

var thermal = []status.Thermal{
	{Name: "CPU", Zone: "deca-cpu-max-step"},
	{Name: "GPU", Zone: "gpu0-usr"},
}

func page() status.Page {
	c := config.Get()
	r := metrics.Reader{}

	return status.Page{
		Name:    c.Device.Name,
		Model:   c.Device.Model,
		Version: layout.VersionString(),
		Adopted: Adopted(),
		Parts:   status.Parts(component.Default().Progress()),
		Groups: []status.Group{
			status.Network(wifi.Get().Network(), wifi.Get().MAC(), r),
			status.Clock(c.Time.Chosen, c.Time.Home),
			status.Screen(c.Screen),
			volumes(c),
			status.System(r, layout.StateDir, thermal),
		},
	}
}

func volumes(c config.Config) status.Group {
	rows := make([]status.Row, 0, 4)
	for _, s := range []config.Stream{
		config.StreamMedia, config.StreamAlerts, config.StreamVoice, config.StreamFeedback,
	} {
		rows = append(rows, status.Row{Name: s.Label(), Value: status.Percent(c.Volume.Level(s))})
	}
	return status.Group{Title: "Volume", Rows: rows}
}
