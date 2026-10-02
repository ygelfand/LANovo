package speaker

import (
	"math"
	"testing"
)

// The ends are what somebody notices first: nought has to be silence and a hundred has to be
// everything, whatever the curve does in between.
func TestGainRunsFromSilenceToFull(t *testing.T) {
	if got := Gain(0); got != 0 {
		t.Errorf("nought is %v, want 0", got)
	}
	if got := Gain(100); math.Abs(float64(got)-1) > 1e-6 {
		t.Errorf("a hundred is %v, want 1", got)
	}
}

// The points come from the device's own table, so they are worth holding: a typo in one of them is
// a level that sounds wrong and nothing that says so.
func TestGainMatchesTheCurvesPoints(t *testing.T) {
	for _, at := range curve {
		want := math.Pow(10, float64(at.mB)/2000)
		if got := Gain(at.at); math.Abs(float64(got)-want) > 1e-6 {
			t.Errorf("at %d the gain is %v, want %v", at.at, got, want)
		}
	}
}

// Louder has to mean louder at every step, or a slider drifts back and forth as it is dragged.
func TestGainOnlyRises(t *testing.T) {
	last := float32(-1)
	for percent := range 101 {
		got := Gain(percent)
		if got < last {
			t.Fatalf("at %d the gain fell to %v from %v", percent, got, last)
		}
		last = got
	}
}

// Half way should be well down, not nearly full: that it is not is the whole reason for the curve.
func TestHalfWayIsWellDown(t *testing.T) {
	if got := Gain(50); got > 0.15 {
		t.Errorf("half way is %v, which is louder than the curve intends", got)
	}
}
