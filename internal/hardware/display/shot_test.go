package display

import (
	"strings"
	"testing"
	"time"
)

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

	dash := d.Claim(PriorityDashboard)
	dash.Show(noop)
	boot.Release()

	if at, _ := d.Showing(); at != PriorityDashboard {
		t.Errorf("showing %v after the boot screen let go, want the dashboard", at)
	}
}

func TestAClaimWithNothingToDrawIsNotShowing(t *testing.T) {
	d := NewDriver("/dev/null")
	d.Claim(PriorityUI)

	if _, showing := d.Showing(); showing {
		t.Error("an empty claim counted as showing")
	}
}

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
