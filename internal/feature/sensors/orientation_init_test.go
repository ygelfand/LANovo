package sensors

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/display/geometry"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

func TestMountedOrientationResolvesAfterEarlyConstruction(t *testing.T) {
	wasBoard, wasRotation := board.Current(), display.Get().Orientation()
	t.Cleanup(func() { board.Set(wasBoard); display.Get().SetOrientation(wasRotation) })
	board.Set(board.Blueberry)
	s := Get()
	board.Set(board.Ivy)
	s.resetMounted()
	if got := display.Get().Orientation(); got != geometry.Rotate270 {
		t.Fatalf("Ivy inherited constructor orientation: %v", got)
	}
	if got := s.Orientation(); got != geometry.Rotate270 {
		t.Fatalf("orientation tracker inherited constructor orientation: %v", got)
	}
}
