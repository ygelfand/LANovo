package settings

import (
	sharedpages "github.com/ygelfand/libcountertop/pkg/display/settings"

	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/shell"
)

var clocks = sharedpages.NewClockPages(
	sharedpages.ClockDependencies{
		Preferences: preferences(),
		Shell:       shell.Get(),
		Controller:  dashboard.Get(),
		Previews:    previewKit(),
	},
)
