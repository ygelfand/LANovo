package display

import (
	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/libcountertop/pkg/display/geometry"
)

type Orientation = geometry.Orientation

const (
	Rotate0   = geometry.Rotate0
	Rotate90  = geometry.Rotate90
	Rotate180 = geometry.Rotate180
	Rotate270 = geometry.Rotate270
)

func Mounted() Orientation { return Orientation(board.Current().Mounted) }
