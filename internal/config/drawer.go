package config

import "github.com/ygelfand/LANovo/internal/lib/say"

// Edge is a side of the picture, not of the panel: the device gets stood in different places and
// turned, and a right-handed reach is not a left-handed one.
type Edge string

const (
	EdgeLeft   Edge = "left"
	EdgeRight  Edge = "right"
	EdgeTop    Edge = "top"
	EdgeBottom Edge = "bottom"
)

// DefaultEdge is the right, which is where a hand reaching past the screen already is.
const DefaultEdge = EdgeRight

// Label is how the setting is shown.
func (e Edge) Label() string {
	switch e {
	case EdgeLeft:
		return say.T("edge.left")
	case EdgeRight:
		return say.T("edge.right")
	case EdgeTop:
		return say.T("edge.top")
	case EdgeBottom:
		return say.T("edge.bottom")
	}
	return string(e)
}

// Edges is every value the setting takes.
func Edges() []Edge { return []Edge{EdgeLeft, EdgeRight, EdgeTop, EdgeBottom} }

// Sides is the two a column can hang off. A control that is a column against the top or the bottom
// is a different control rather than the same one turned, so those are not offered rather than
// offered and quietly treated as the right.
func Sides() []Edge { return []Edge{EdgeLeft, EdgeRight} }
