package video

import (
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/lib/surface"
)

func TestUprightVideoIsScaledIntoPlace(t *testing.T) {
	x, y, m, at := placement(display.Rotate0, 1200, 1920, 1920, 1080)
	if x != 0 || y != 622 || m != (surface.Matrix{DsDx: 0.625, DsDy: 0.625}) {
		t.Errorf("position %v,%v matrix %+v", x, y, m)
	}
	if at != (display.Rect{X: 0, Y: 622, W: 1200, H: 675}) {
		t.Errorf("at %+v", at)
	}
}

func TestSidewaysVideoIsTurnedIntoPlace(t *testing.T) {
	x, y, m, at := placement(display.Mounted(), 1200, 1920, 1920, 1080)
	if x != 60 || y != 1920 || m != (surface.Matrix{DtDx: -1, DtDy: 1}) {
		t.Errorf("position %v,%v matrix %+v", x, y, m)
	}
	if at != (display.Rect{X: 60, Y: 0, W: 1080, H: 1920}) {
		t.Errorf("at %+v", at)
	}
}

func TestItFillsThePanelWithoutStretching(t *testing.T) {
	for _, c := range []struct {
		w, h int
		want display.Rect
	}{
		{1080, 1920, display.Rect{X: 60, Y: 0, W: 1080, H: 1920}},
		{1920, 1080, display.Rect{X: 0, Y: 622, W: 1200, H: 675}},
		{1200, 1920, display.Rect{W: 1200, H: 1920}},
	} {
		if got := fit(1200, 1920, c.w, c.h); got != c.want {
			t.Errorf("%dx%d on 1200x1920: %+v, want %+v", c.w, c.h, got, c.want)
		}
	}
}

func TestAStreamThatRunsDryIsStalled(t *testing.T) {
	t0 := time.Unix(0, 0)
	var w stallWatch
	w.show(t0)
	w.stalled(t0, time.Second, true, 1, 0, false)

	if w.stalled(t0.Add(300*time.Millisecond), 1300*time.Millisecond, true, 1, 0, false) {
		t.Error("stalled before the wait was up")
	}
	if !w.stalled(t0.Add(500*time.Millisecond), 1500*time.Millisecond, true, 1, 0, false) {
		t.Error("no pictures for 500ms while the sound played on, and not stalled")
	}
	w.show(t0.Add(600 * time.Millisecond))
	if w.stalled(t0.Add(700*time.Millisecond), 1700*time.Millisecond, true, 2, 0, false) {
		t.Error("still stalled after a picture")
	}
}

func TestAClockThatStopsWhilePlayingIsStalled(t *testing.T) {
	t0 := time.Unix(0, 0)
	var w stallWatch
	w.show(t0)
	w.stalled(t0, time.Second, true, 1, 3, false)

	if w.stalled(t0.Add(300*time.Millisecond), time.Second, true, 1, 3, false) {
		t.Error("stalled before the wait was up")
	}
	if !w.stalled(t0.Add(500*time.Millisecond), time.Second, true, 1, 3, false) {
		t.Error("the clock stood still for 500ms with pictures waiting, and not stalled")
	}
}

func TestPicturesWaitingOnAMovingClockAreNotAStall(t *testing.T) {
	t0 := time.Unix(0, 0)
	var w stallWatch
	w.show(t0)
	for i := range 10 {
		at := time.Duration(i) * 100 * time.Millisecond
		if w.stalled(t0.Add(at), time.Second+at, true, 1, 2, false) {
			t.Fatalf("a slow picture rate stalled at %v", at)
		}
	}
}

func TestPausedStartingOrFinishedIsNotAStall(t *testing.T) {
	t0 := time.Unix(0, 0)
	later := t0.Add(2 * time.Second)

	var w stallWatch
	w.show(t0)
	if w.stalled(later, time.Second, false, 1, 0, false) {
		t.Error("a pause stalled")
	}
	if w.stalled(later.Add(300*time.Millisecond), time.Second, true, 1, 3, false) {
		t.Error("stalled the moment it resumed")
	}

	var fresh stallWatch
	if fresh.stalled(later, time.Second, true, 0, 0, false) {
		t.Error("stalled before the first picture")
	}
	if fresh.stalled(later, time.Second, true, 5, 0, true) {
		t.Error("stalled at the end of the stream")
	}
}
