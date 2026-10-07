package dashboard

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

func TestTheClockSitsWhereItIsPut(t *testing.T) {
	const w, h = 1920, 1200
	left := Place(config.PositionTop, config.AlignLeft, config.SizeSmall, w, h)
	right := Place(config.PositionBottom, config.AlignRight, config.SizeSmall, w, h)
	center := Place(config.PositionCenter, config.AlignCenter, config.SizeSmall, w, h)

	if left.X != 0 || left.Y != 0 {
		t.Errorf("top left at %v", left)
	}
	if right.X+right.W != w || right.Y+right.H != h {
		t.Errorf("bottom right at %v", right)
	}
	if d, e := w-(center.X*2+center.W), h-(center.Y*2+center.H); d < 0 || d > 1 || e < 0 || e > 1 {
		t.Errorf("center at %v", center)
	}
	if Place(
		config.PositionCenter,
		config.AlignCenter,
		config.SizeLarge,
		w,
		h,
	) != Box(
		config.PositionCenter,
		config.SizeLarge,
		w,
		h,
	) {
		t.Error("centered placement differs from the dashboard's box")
	}
}
