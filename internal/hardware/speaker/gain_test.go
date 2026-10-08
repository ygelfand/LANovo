package speaker

import (
	"math"
	"testing"
)

func TestGainRunsFromSilenceToFull(t *testing.T) {
	if got := Gain(0); got != 0 {
		t.Errorf("nought is %v, want 0", got)
	}
	if got := Gain(100); math.Abs(float64(got)-1) > 1e-6 {
		t.Errorf("a hundred is %v, want 1", got)
	}
}

func TestGainMatchesTheCurvesPoints(t *testing.T) {
	for _, at := range curve {
		want := math.Pow(10, float64(at.mB)/2000)
		if got := Gain(at.at); math.Abs(float64(got)-want) > 1e-6 {
			t.Errorf("at %d the gain is %v, want %v", at.at, got, want)
		}
	}
}

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

func TestHalfWayIsWellDown(t *testing.T) {
	if got := Gain(50); got > 0.15 {
		t.Errorf("half way is %v, which is louder than the curve intends", got)
	}
}
