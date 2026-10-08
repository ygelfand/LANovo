package display

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestALastFrameWithNothingRenderingGivesUp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := NewDriver("/dev/null").Last(ctx, func(*Panel) error { return nil })
	if err == nil {
		t.Fatal("a frame nothing could draw was reported as up")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("gave up with %v, want the deadline", err)
	}
}

func TestAFrameThatDidNotGoUpDoesNotStopRendering(t *testing.T) {
	d := NewDriver("/dev/null")

	if err := d.paint(func(*Panel) error { return nil }); err == nil {
		t.Fatal("painting with no panel open reported success")
	}

	d.mu.Lock()
	parting := d.parting
	d.mu.Unlock()

	if parting {
		t.Error("rendering stopped for a frame that never reached the panel")
	}
}

func TestNothingRendersOverTheLastFrame(t *testing.T) {
	d := NewDriver("/dev/null")

	d.mu.Lock()
	d.parting = true
	d.mu.Unlock()

	c := d.Claim(PriorityBoot)

	var drawn bool
	c.Show(func(*Panel) error { drawn = true; return nil })

	if err := d.render(); err != nil {
		t.Fatalf("render: %v", err)
	}
	if drawn {
		t.Error("a claim was drawn after the last frame went up")
	}
}
