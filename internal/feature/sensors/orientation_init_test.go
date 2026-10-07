package sensors

import (
	"testing"

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
	if got := display.Get().Orientation(); got != display.Rotate270 {
		t.Fatalf("Ivy inherited constructor orientation: %v", got)
	}
	if got := s.Orientation(); got != display.Rotate270 {
		t.Fatalf("orientation tracker inherited constructor orientation: %v", got)
	}
}
