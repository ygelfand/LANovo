package display

import (
	"strings"
	"testing"
	"time"
)

// Nothing is on the panel before anything asks for it, and a harness waiting for the device to
// come up has to be able to tell that from the boot screen holding it.
func TestShowingSaysWhatHasThePanel(t *testing.T) {
	d := NewDriver("/dev/null")

	if _, showing := d.Showing(); showing {
		t.Error("something is showing before anything claimed the panel")
	}

	boot := d.Claim(PriorityBoot)
	boot.Show(noop)

	at, showing := d.Showing()
	if !showing {
		t.Fatal("nothing is showing with the boot screen up")
	}
	if at != PriorityBoot {
		t.Errorf("showing %v, want the boot screen", at)
	}

	// Which is the thing being waited for: the boot screen letting go.
	dash := d.Claim(PriorityDashboard)
	dash.Show(noop)
	boot.Release()

	if at, _ := d.Showing(); at != PriorityDashboard {
		t.Errorf("showing %v after the boot screen let go, want the dashboard", at)
	}
}

// A claim with nothing to draw does not have the panel, which is the same rule the render loop
// follows: a claim is a place in the order, not a picture.
func TestAClaimWithNothingToDrawIsNotShowing(t *testing.T) {
	d := NewDriver("/dev/null")
	d.Claim(PriorityUI)

	if _, showing := d.Showing(); showing {
		t.Error("an empty claim counted as showing")
	}
}

// The screenshot is taken by the render loop, so asking when nothing is rendering has to come back
// and say so rather than wait for a loop that will never answer.
func TestAScreenshotWithNothingRenderingGivesUp(t *testing.T) {
	was := shotWait
	shotWait = 50 * time.Millisecond
	t.Cleanup(func() { shotWait = was })

	done := make(chan error, 1)
	go func() {
		_, _, _, err := NewDriver("/dev/null").Shot()
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a screenshot was taken with nothing rendering")
		}
		if !strings.Contains(err.Error(), "rendering") {
			t.Errorf("error is %q, and does not say why", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Shot did not come back, so it is waiting for a loop that is not there")
	}
}
