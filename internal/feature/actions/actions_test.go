package actions

import (
	"slices"
	"testing"
)

func TestAlarmsAndCallControlsAreRegistered(t *testing.T) {
	names := Get().Names()
	for _, name := range []string{"alarm", "call", "call_answer", "call_hangup", "broadcast"} {
		if !slices.Contains(names, name) {
			t.Errorf("missing action %s in %v", name, names)
		}
	}
}
