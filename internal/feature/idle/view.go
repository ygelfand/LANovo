package idle

import (
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/dashboard/face"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/gpu"
	sharedview "github.com/ygelfand/libcountertop/pkg/display/idleview"
)

type View = sharedview.View
type Piece = sharedview.Piece

var Areas = sharedview.Areas
var Pieces = sharedview.Pieces

func newView() *View {
	return sharedview.New(sharedview.Dependencies{
		Read: func() sharedview.Settings {
			c := config.Get()
			return sharedview.Settings{
				Screen: c.Screen,
				Clock:  c.Clock,
				Idle:   c.Idle,
				Visual: c.Visual,
			}
		},
		Visuals: visuals.Get(), GPU: gpu.Get(), Display: display.Get(), Shell: shell.Get(),
	})
}
func Reading(c config.Config, at time.Time) face.Reading {
	return sharedview.Reading(
		sharedview.Settings{Screen: c.Screen, Clock: c.Clock, Idle: c.Idle, Visual: c.Visual},
		at,
	)
}
