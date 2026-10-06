package widget

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

type Kind int

const (
	Plain Kind = iota
	Chevron
	Toggle
	Slider
	Field
)

type Row struct {
	Glyph string
	Label string
	Hint  string

	Snap func(level int) int

	Value string

	Kind  Kind
	On    bool
	Level int

	Chosen bool
	Dim    bool

	Preview func(s ui.Surface, at ui.Rect, palette theme.Theme)

	Save func(string)
}

type Cell struct {
	Label  string
	Chosen bool

	Paint func(s ui.Surface, at ui.Rect, palette theme.Theme)

	Palette *theme.Theme

	Face config.Face

	Style string

	Weather config.WeatherLook
}
