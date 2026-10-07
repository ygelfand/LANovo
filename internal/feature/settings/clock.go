package settings

import (
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	sharedpages "github.com/ygelfand/libcountertop/pkg/display/settings"
)

var clocks = sharedpages.NewClockPages(sharedpages.ClockDependencies{Preferences: preferences(), Shell: shell.Get(), Controller: dashboard.Get()})
var clockPage = clocks.Page
var facePage = clocks.Faces
var preview = clocks.Preview
var positionPage = clocks.Positions
var sizePage = clocks.Sizes
var colorPage = clocks.Colors
