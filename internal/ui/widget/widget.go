package widget

import (
	"github.com/ygelfand/libcountertop/pkg/display/model"
)

type Kind = model.Kind
type Row = model.Row
type Cell = model.Cell

const (
	Plain   = model.Plain
	Chevron = model.Chevron
	Toggle  = model.Toggle
	Slider  = model.Slider
	Field   = model.Field
)
