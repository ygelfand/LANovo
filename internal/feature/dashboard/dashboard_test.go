package dashboard

import (
	"testing"
)

func TestTheMarkSitsAtTheFootEitherWayUp(t *testing.T) {
	land, port := Mark(1920, 1200), Mark(1200, 1920)

	if land.H != port.H {
		t.Errorf("the mark is %d tall landscape and %d portrait", land.H, port.H)
	}
	if got := 1200 - (land.Y + land.H); got != 1920-(port.Y+port.H) {
		t.Errorf("the gap below is %d landscape and %d portrait", got, 1920-(port.Y+port.H))
	}
	if land.Y+land.H > 1200 || port.Y+port.H > 1920 {
		t.Error("the mark runs off the bottom")
	}
}
