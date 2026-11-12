package board

import "testing"

func TestX501FVolumeButtons(t *testing.T) {
	want := map[string]int{"volume up": 85, "volume down": 1019}
	for _, button := range X501F.Buttons {
		gpio, ok := want[button.Control]
		if !ok || gpio != button.GPIO || button.ActiveLow {
			t.Fatalf("unexpected X501F button: %+v", button)
		}
		delete(want, button.Control)
	}
	if len(want) != 0 || len(X501F.Keys) != 0 {
		t.Fatalf("missing buttons or duplicate evdev mapping: missing=%v keys=%v", want, X501F.Keys)
	}
}
