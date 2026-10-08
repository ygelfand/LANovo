package gui

import (
	sharedlib "github.com/ygelfand/libcountertop/pkg/display/widgets"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

type paint = sharedlib.Paint

func painted(key string, w, h int, fill theme.Color, draw paint) string {
	return sharedlib.Painted(key, w, h, fill, presentation.Palette(), draw)
}

var picture = sharedlib.Picture
