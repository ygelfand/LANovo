package config

import "testing"

// Home Assistant can set slot 2 on a device where slot 1 was never touched. The slots invented to
// reach it have to come up with the defaults in them: a zeroed threshold fires on silence, and a
// zeroed listening limit ends a turn before anyone has spoken.
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

// An unset slot reads as the defaults rather than as zeros, whether or not the list has reached it.
func TestAnUnsetSlotIsTheDefaults(t *testing.T) {
	if got := (Wake{}).Slot(0); got != DefaultWakeWord() {
		t.Errorf("an untouched slot reads %+v", got)
	}
}

// The stop word is off at the top of its range, so there is no second control saying the same thing.
func TestTheStopWordIsOffAtTheTop(t *testing.T) {
	if (Stop{Threshold: StopOff}).Listening() {
		t.Error("the stop word listens at the threshold that turns it off")
	}
	if !(Stop{Threshold: DefaultStopThreshold}).Listening() {
		t.Error("the stop word is off by default")
	}
}
