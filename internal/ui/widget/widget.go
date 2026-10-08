package widget

import (
	"github.com/ygelfand/libcountertop/pkg/display/menu"
)

type Kind = menu.Kind
type Row = menu.Row
type Cell = menu.Cell

const (
	Plain   = menu.Plain
	Chevron = menu.Chevron
	Toggle  = menu.Toggle
	Slider  = menu.Slider
	Field   = menu.Field
)
