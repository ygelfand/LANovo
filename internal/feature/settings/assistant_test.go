package settings

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
)

func TestTheSensitivitySliderIsTheThresholdTurnedAround(t *testing.T) {
	for _, tc := range []struct {
		was  float64
		want int
	}{
		{0.85, 64},
		{0.5, mostSure},
		{0.99, leastSure},
		{0, mostSure},
		{1, leastSure},
		{2, leastSure},
	} {
		if got := sensitivity(tc.was); got != tc.want {
			t.Errorf("a threshold of %v shows as %d, want %d", tc.was, got, tc.want)
		}
	}
}

// Dragging to either end saves something the engine can use rather than a number that turns
// detection off or makes it fire on silence.
func TestDraggingToTheEndsSavesAUsableThreshold(t *testing.T) {
	config.Use(t.TempDir() + "/state.json")

	for _, level := range []int{0, 50, 85, 99, 100} {
		setThreshold(0)(level)

		got := config.Get().Wake.Slot(0).Threshold
		if got < 0.5 || got > 0.99 {
			t.Errorf("dragging to %d saved %v, outside what a model can use", level, got)
		}
	}
}

// What a slider shows and what it saves have to agree, or a page redrawn after a drag jumps.
func TestTheSliderShowsWhatItSaved(t *testing.T) {
	config.Use(t.TempDir() + "/state.json")

	for _, level := range []int{50, 70, 85, 99} {
		setThreshold(1)(level)

		if got := sensitivity(config.Get().Wake.Slot(1).Threshold); got != level {
			t.Errorf("dragged to %d, reads back as %d", level, got)
		}
	}
}

// Every slot has a page, and each writes to its own.
func TestEachSlotSetsItsOwnThreshold(t *testing.T) {
	config.Use(t.TempDir() + "/state.json")

	setThreshold(0)(60)
	setThreshold(1)(90)

	if got := config.Get().Wake.Slot(0).Threshold; got != 0.89 {
		t.Errorf("slot 1 is at %v after only slot 2 was changed", got)
	}
	if got := config.Get().Wake.Slot(1).Threshold; got != 0.59 {
		t.Errorf("slot 2 is at %v", got)
	}
	if wakeword.Slots != 2 {
		t.Errorf("%d slots, and this test assumes two", wakeword.Slots)
	}
}
