package display

import "github.com/ygelfand/LANovo/internal/board"

// Orientation is how the picture sits on the panel, as a rotation from the panel's own portrait.
//
// Every drawing surface works in viewed coordinates and every touch arrives in panel coordinates,
// so this is the one place the two are reconciled. Nothing above here knows which way up the
// device is.
type Orientation int

const (
	// Rotate0 is the panel's native portrait, origin top-left.
	Rotate0 Orientation = 0

	Rotate90  Orientation = 90
	Rotate180 Orientation = 180
	Rotate270 Orientation = 270
)

// Mounted is how this board stands on a shelf.
func Mounted() Orientation { return Orientation(board.Current().Mounted) }

func (o Orientation) String() string {
	switch o {
	case Rotate0:
		return "portrait"
	case Rotate90:
		return "landscape"
	case Rotate180:
		return "portrait inverted"
	case Rotate270:
		return "landscape inverted"
	}
	return "unknown"
}

// Sideways reports whether the rotation swaps width and height.
func (o Orientation) Sideways() bool { return o == Rotate90 || o == Rotate270 }

// Size is what a panel of this size looks like at this rotation.
func (o Orientation) Size(fbW, fbH int) (w, h int) {
	if o.Sideways() {
		return fbH, fbW
	}
	return fbW, fbH
}

// Project turns a viewed position into a position in the framebuffer.
func (o Orientation) Project(fbW, fbH, vx, vy int) (x, y int) {
	switch o {
	case Rotate90:
		return vy, fbH - 1 - vx
	case Rotate180:
		return fbW - 1 - vx, fbH - 1 - vy
	case Rotate270:
		return fbW - 1 - vy, vx
	}
	return vx, vy
}

// Unproject turns a position in the framebuffer back into a viewed one, which is how a touch
// lands where it was drawn.
func (o Orientation) Unproject(fbW, fbH, x, y int) (vx, vy int) {
	switch o {
	case Rotate90:
		return fbH - 1 - y, x
	case Rotate180:
		return fbW - 1 - x, fbH - 1 - y
	case Rotate270:
		return y, fbW - 1 - x
	}
	return x, y
}
