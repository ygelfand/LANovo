package boot

import (
	"context"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Where the word is drawn, worked out the same way drawLeaving works it out.
func wordBand(w, h int) ui.Rect {
	short := min(w, h)
	mark := int(float64(short) * leavingMarkShare)

	font := ui.MustLoad(ui.Medium, int(float64(short)*leavingWordShare))
	_, wordHeight := font.Measure(Leaving())

	top := (h - (mark + wordHeight + wordHeight)) / 2
	return ui.Rect{Y: top + mark + wordHeight, W: w, H: wordHeight}
}

// Both ways up, since the panel is stood either way and this is the last thing it shows.
func TestDrawLeavingFitsBothWaysUp(t *testing.T) {
	for _, s := range sizes {
		img := ui.NewImage(s.w, s.h, theme.Brand().Surface)

		if err := drawLeaving(img); err != nil {
			t.Fatalf("%s: drawLeaving: %v", s.name, err)
		}
	}
}

// The word is the whole point: without it this is the boot logo and says the wrong thing.
func TestDrawLeavingWritesTheWord(t *testing.T) {
	for _, s := range sizes {
		blank := ui.NewImage(s.w, s.h, theme.Brand().Surface)
		ui.Fill(blank, chosen().Background)

		img := ui.NewImage(s.w, s.h, theme.Brand().Surface)
		if err := drawLeaving(img); err != nil {
			t.Fatalf("%s: drawLeaving: %v", s.name, err)
		}

		if same(blank, img, wordBand(s.w, s.h)) {
			t.Errorf("%s: nothing is drawn where the word goes", s.name)
		}
	}
}

// A restart that looked exactly like a cold boot would be the fault this is fixing.
func TestLeavingDoesNotLookLikeTheBootLogo(t *testing.T) {
	coming := ui.NewImage(1200, 1920, theme.Brand().Surface)
	if err := drawLogo(coming); err != nil {
		t.Fatalf("drawLogo: %v", err)
	}

	going := ui.NewImage(1200, 1920, theme.Brand().Surface)
	if err := drawLeaving(going); err != nil {
		t.Fatalf("drawLeaving: %v", err)
	}

	if same(coming, going, ui.Rect{W: 1200, H: 1920}) {
		t.Error("the restarting screen is pixel for pixel the boot logo")
	}
}

// The shutdown must not wait on a panel that is never going to take the frame. There is no render
// loop in a test, which is the same position a device with no display is in.
func TestLeavingGivesUpOnAPanelThatCannotTakeIt(t *testing.T) {
	was := leavingWait
	leavingWait = 50 * time.Millisecond
	t.Cleanup(func() { leavingWait = was })

	ctx, stop := context.WithCancel(context.Background())
	group := leaving(ctx)

	select {
	case <-group.Done():
		t.Fatal("the group was told to stop before anything asked it to")
	default:
	}

	stop()

	select {
	case <-group.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the group was never told to stop, so the shutdown would hang")
	}
}

// The group keeps running while the frame is going up: stopping first is what would take the
// display down before it could paint.
func TestLeavingHoldsTheGroupUntilTheFrameIsDone(t *testing.T) {
	was := leavingWait
	leavingWait = 250 * time.Millisecond
	t.Cleanup(func() { leavingWait = was })

	ctx, stop := context.WithCancel(context.Background())
	group := leaving(ctx)

	stop()

	select {
	case <-group.Done():
		t.Error("the group stopped immediately, so nothing had time to be painted")
	case <-time.After(100 * time.Millisecond):
	}
}
