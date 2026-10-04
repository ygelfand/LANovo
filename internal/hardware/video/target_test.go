package video

import "testing"

func TestTheTargetFollowsTheWayThePanelIsTurned(t *testing.T) {
	for _, c := range []struct{ w, h, tallest int }{
		{1920, 1200, 1080},
		{1200, 1920, 720},
		{1280, 800, 720},
		{800, 1280, 480},
		{3840, 2160, 1080},
	} {
		got := target(c.w, c.h, 60)
		if got.Tallest != c.tallest || got.Width != c.w || got.Height != c.h || got.Fastest != 60 {
			t.Errorf("%dx%d: %+v, want tallest %d", c.w, c.h, got, c.tallest)
		}
	}
}
