package display

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/board"
)

// The panel, as this device has it.
const (
	fbW = 1200
	fbH = 1920
)

func TestSize(t *testing.T) {
	tests := []struct {
		name         string
		rot          Orientation
		wantW, wantH int
	}{
		{"portrait is the panel itself", Rotate0, fbW, fbH},
		{"landscape swaps them", Rotate90, fbH, fbW},
		{"upside down does not", Rotate180, fbW, fbH},
		{"landscape inverted swaps them", Rotate270, fbH, fbW},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h := tt.rot.Size(fbW, fbH)
			if w != tt.wantW || h != tt.wantH {
				t.Errorf("Size() = %dx%d, want %dx%d", w, h, tt.wantW, tt.wantH)
			}
		})
	}
}

// Every viewed pixel must land somewhere in the framebuffer, at every rotation. One that does not
// is a row or column drawn off the edge.
func TestProjectStaysInTheFramebuffer(t *testing.T) {
	for _, rot := range []Orientation{Rotate0, Rotate90, Rotate180, Rotate270} {
		w, h := rot.Size(fbW, fbH)

		for _, v := range [][2]int{{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1}} {
			x, y := rot.Project(fbW, fbH, v[0], v[1])
			if x < 0 || x >= fbW || y < 0 || y >= fbH {
				t.Errorf("%v: Project(%d, %d) = %d,%d, outside %dx%d", rot, v[0], v[1], x, y, fbW, fbH)
			}
		}
	}
}

// Unproject is what makes a touch land where it was drawn, so it has to be the exact inverse at
// every rotation. Anything else and taps drift as the device turns.
func TestUnprojectIsTheInverseOfProject(t *testing.T) {
	for _, rot := range []Orientation{Rotate0, Rotate90, Rotate180, Rotate270} {
		w, h := rot.Size(fbW, fbH)

		for _, v := range [][2]int{{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1}, {w / 2, h / 2}, {7, 13}} {
			x, y := rot.Project(fbW, fbH, v[0], v[1])
			gx, gy := rot.Unproject(fbW, fbH, x, y)

			if gx != v[0] || gy != v[1] {
				t.Errorf("%v: %d,%d projected to %d,%d and came back %d,%d",
					rot, v[0], v[1], x, y, gx, gy)
			}
		}
	}
}

// Distinct viewed pixels must not land on the same framebuffer pixel, or the picture folds.
func TestProjectIsOneToOne(t *testing.T) {
	for _, rot := range []Orientation{Rotate0, Rotate90, Rotate180, Rotate270} {
		w, h := rot.Size(fbW, fbH)
		seen := map[[2]int]bool{}

		// A stride, rather than every pixel: two million points per rotation says nothing more
		// than a few thousand spread across the panel.
		for vx := 0; vx < w; vx += 7 {
			for vy := 0; vy < h; vy += 11 {
				at := [2]int{}
				at[0], at[1] = rot.Project(fbW, fbH, vx, vy)

				if seen[at] {
					t.Fatalf("%v: two viewed pixels land on %v", rot, at)
				}
				seen[at] = true
			}
		}
	}
}

// Rotate0 is the identity, which is what makes it the one to reason about the others against.
func TestRotate0IsIdentity(t *testing.T) {
	for _, v := range [][2]int{{0, 0}, {5, 9}, {fbW - 1, fbH - 1}} {
		x, y := Rotate0.Project(fbW, fbH, v[0], v[1])
		if x != v[0] || y != v[1] {
			t.Errorf("Rotate0 moved %d,%d to %d,%d", v[0], v[1], x, y)
		}
	}
}

func TestMounted(t *testing.T) {
	if Mounted() != Rotate90 {
		t.Fatalf("Mounted is %v, want %v", Mounted(), Rotate90)
	}

	w, h := Mounted().Size(fbW, fbH)
	if w != 1920 || h != 1200 {
		t.Errorf("mounted size is %dx%d, want 1920x1200", w, h)
	}

	if x, y := Mounted().Project(fbW, fbH, 0, 0); x != 0 || y != fbH-1 {
		t.Errorf("the viewed origin projects to %d,%d, want 0,%d", x, y, fbH-1)
	}
}

func TestIvyIsMountedTheOtherWayUp(t *testing.T) {
	board.Set(board.Ivy)
	defer board.Set(board.Blueberry)
	if Mounted() != Rotate270 {
		t.Errorf("ivy is mounted %v, want %v", Mounted(), Rotate270)
	}
}

func TestOrientationNames(t *testing.T) {
	for _, tt := range []struct {
		rot  Orientation
		want string
	}{
		{Rotate0, "portrait"},
		{Rotate90, "landscape"},
		{Rotate180, "portrait inverted"},
		{Rotate270, "landscape inverted"},
	} {
		if got := tt.rot.String(); got != tt.want {
			t.Errorf("%d.String() = %q, want %q", int(tt.rot), got, tt.want)
		}
	}
}

// The driver holds the rotation whether or not a panel is open, so a device turned before the
// panel is taken still comes up the right way round.
func TestDriverHoldsOrientationWithNoPanel(t *testing.T) {
	d := NewDriver("/dev/null")

	if got := d.Orientation(); got != Mounted() {
		t.Errorf("a new driver is at %v, want %v", got, Mounted())
	}

	d.SetOrientation(Rotate180)
	if got := d.Orientation(); got != Rotate180 {
		t.Errorf("after turning, %v, want %v", got, Rotate180)
	}
}

// Turning to the rotation already shown must not wake the render loop, or a device resting on a
// boundary redraws forever.
func TestSettingTheSameOrientationDoesNothing(t *testing.T) {
	d := NewDriver("/dev/null")
	d.SetOrientation(Rotate180)

	// Drain whatever the first change queued.
	select {
	case <-d.changed:
	default:
	}

	d.SetOrientation(Rotate180)

	select {
	case <-d.changed:
		t.Error("setting the same orientation asked for a redraw")
	default:
	}
}

// Turning a panel swaps what it calls its width and height, which is what everything drawing on
// it works in.
func TestTurnResizesTheViewedPanel(t *testing.T) {
	p := &Panel{fbW: fbW, fbH: fbH}

	p.Turn(Rotate90)
	if p.Width != fbH || p.Height != fbW {
		t.Errorf("landscape is %dx%d, want %dx%d", p.Width, p.Height, fbH, fbW)
	}

	p.Turn(Rotate0)
	if p.Width != fbW || p.Height != fbH {
		t.Errorf("portrait is %dx%d, want %dx%d", p.Width, p.Height, fbW, fbH)
	}
}
