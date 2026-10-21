package ui

import (
	sharedui "github.com/ygelfand/libcountertop/pkg/display/ui"

	"github.com/ygelfand/LANovo/internal/brand"
)

var logo = sharedui.NewLogo(brand.LogoLight, brand.LogoDark)

func Logo() *sharedui.Logo { return logo }
