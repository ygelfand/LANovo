package ui

import (
	_ "embed"

	sharedui "github.com/ygelfand/libcountertop/pkg/display/ui"
)

//go:embed logo_light.png
var lightPNG []byte

//go:embed logo_dark.png
var darkPNG []byte

var logo = sharedui.NewLogo(lightPNG, darkPNG)

func Logo() *sharedui.Logo { return logo }
