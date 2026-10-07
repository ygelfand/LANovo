package settings

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	sharedpages "github.com/ygelfand/libcountertop/pkg/display/settings"
)

var clocks = sharedpages.NewClockPages(sharedpages.ClockOptions{
	Read: func() sharedpages.ClockState {
		c := config.Get()
		return sharedpages.ClockState{Screen: c.Screen, Clock: c.Clock}
	},
	Push:     func(v shell.View) { shell.Get().Push(v) },
	Face:     func(v config.Face) { dashboard.Get().SetFace(v) },
	Position: func(v config.Position) { dashboard.Get().SetPosition(v) },
	Size:     func(v config.Size) { dashboard.Get().SetSize(v) },
	Ink:      func(v config.Ink) { dashboard.Get().SetInk(v) },
	Date:     func(v bool) { dashboard.Get().SetDate(v) },
})
var clockPage = clocks.Page
var facePage = clocks.Faces
var preview = clocks.Preview
var positionPage = clocks.Positions
var sizePage = clocks.Sizes
var colorPage = clocks.Colors
