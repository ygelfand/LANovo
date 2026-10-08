package idle

import (
	"github.com/ygelfand/libcountertop/pkg/display/idleview"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/gpu"
)

func settings(c config.Config) idleview.Settings {
	return idleview.Settings{Screen: c.Screen, Clock: c.Clock, Idle: c.Idle, Visual: c.Visual}
}

func newView() *idleview.View {
	return idleview.New(idleview.Dependencies{
		Read:    func() idleview.Settings { return settings(config.Get()) },
		Visuals: visuals.Get(), GPU: gpu.Get(), Display: display.Get(), Shell: shell.Get(),
	})
}
