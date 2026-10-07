package gui

import (
	gogui "github.com/go-gui-org/go-gui/gui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	sharedlib "github.com/ygelfand/libcountertop/pkg/display/appearance"
)

type Size = sharedlib.Size

const (
	SizeMini    = sharedlib.SizeMini
	SizeCompact = sharedlib.SizeCompact
	SizeMedium  = sharedlib.SizeMedium
	SizeLarge   = sharedlib.SizeLarge
	SizeXLarge  = sharedlib.SizeXLarge
)

var Sizes = sharedlib.Sizes
var Look = sharedlib.Look

func color(c theme.Color) gogui.Color { return gogui.RGB(c.R, c.G, c.B) }
