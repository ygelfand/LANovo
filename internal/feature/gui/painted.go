package gui

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/libcountertop/pkg/display/style"
	sharedlib "github.com/ygelfand/libcountertop/pkg/display/widgets"
)

type paint = sharedlib.Paint

func palette() theme.Theme {
	s := config.Get().Screen
	if p, ok := theme.ByName(style.Theme(s.Style, s.Theme)); ok {
		return p
	}
	return theme.Default()
}

func painted(key string, w, h int, fill theme.Color, draw paint) string {
	return sharedlib.Painted(key, w, h, fill, palette(), draw)
}

var nrgba = sharedlib.NRGBA
var picture = sharedlib.Picture
