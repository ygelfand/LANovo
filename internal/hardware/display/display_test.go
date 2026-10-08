package display

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/display/geometry"

	"github.com/ygelfand/LANovo/internal/board"
)

const (
	fbW = 1200
	fbH = 1920
)

func TestMounted(t *testing.T) {
	if Mounted() != geometry.Rotate90 {
		t.Fatalf("Mounted is %v, want %v", Mounted(), geometry.Rotate90)
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
	if Mounted() != geometry.Rotate270 {
		t.Errorf("ivy is mounted %v, want %v", Mounted(), geometry.Rotate270)
	}
}

func TestTheConfigFollowsTheBoard(t *testing.T) {
	board.Set(board.Ivy)
	defer board.Set(board.Blueberry)
	c := config()
	if c.Width != board.Ivy.PanelWidth || c.Height != board.Ivy.PanelHeight ||
		c.Mounted != geometry.Rotate270 {
		t.Errorf("ivy's config is %dx%d %v", c.Width, c.Height, c.Mounted)
	}
}
