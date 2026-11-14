package touch

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/display/geometry"

	"github.com/ygelfand/LANovo/internal/board"
)

func TestTouchOrientation(t *testing.T) {
	for _, tc := range []struct {
		ui, x501f geometry.Orientation
	}{
		{geometry.Rotate0, geometry.Rotate90},
		{geometry.Rotate90, geometry.Rotate180},
		{geometry.Rotate180, geometry.Rotate270},
		{geometry.Rotate270, geometry.Rotate0},
	} {
		if got := touchOrientation(board.X501F, tc.ui); got != tc.x501f {
			t.Errorf("X501F touch with UI at %v = %v, want %v", tc.ui, got, tc.x501f)
		}
		for _, b := range board.All() {
			if b.Name != board.X501F.Name && touchOrientation(b, tc.ui) != tc.ui {
				t.Errorf("%s touch changed orientation %v", b.Name, tc.ui)
			}
		}
	}
}
