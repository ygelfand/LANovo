package config

import "testing"

func TestReachingASlotInventsTheOnesBeforeIt(t *testing.T) {
	fresh(t)

	if err := Set().Wake(1).ID("hey_jarvis_v0.1"); err != nil {
		t.Fatalf("setting slot 2: %v", err)
	}

	got := Get().Wake
	if len(got.Words) != 2 {
		t.Fatalf("%d slots, want 2", len(got.Words))
	}
	if got.Slot(0).Threshold != DefaultThreshold {
		t.Errorf("slot 1 came up at %v, want the default", got.Slot(0).Threshold)
	}
	if got.Slot(0).MaxListen != DefaultMaxListen {
		t.Errorf("slot 1 listens for %ds, want the default", got.Slot(0).MaxListen)
	}
	if got.Slot(0).ID != "" {
		t.Errorf("slot 1 is listening for %q, which nobody asked for", got.Slot(0).ID)
	}
}

func TestAnUnsetSlotIsTheDefaults(t *testing.T) {
	if got := (Wake{}).Slot(0); got != DefaultWakeWord() {
		t.Errorf("an untouched slot reads %+v", got)
	}
}

func TestTheStopWordIsOffAtTheTop(t *testing.T) {
	if (Stop{Threshold: StopOff}).Listening() {
		t.Error("the stop word listens at the threshold that turns it off")
	}
	if !(Stop{Threshold: DefaultStopThreshold}).Listening() {
		t.Error("the stop word is off by default")
	}
}
