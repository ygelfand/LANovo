package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A service whose device is not there when the group starts.
//
// The sound card is why this exists: lanovod runs from post-fs-data and ueventd has not always made
// /dev/snd/controlC0 by the time the microphones are acquired. Giving up there left the device with
// no audio until something restarted the whole process, however long the service's restart policy
// said to keep trying.

func TestAServiceNotThereYetIsAskedForAgain(t *testing.T) {
	svc := &fake{name: "microphones", startErr: errors.New("no such device")}

	g := New()
	g.Add(svc, Restart(time.Millisecond, 5*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- g.Run(ctx) }()

	// It has to keep asking rather than give up on the first refusal.
	waitFor(t, func() bool {
		starts, _, _ := svc.counts()
		return starts >= 3
	})

	// The device turns up.
	svc.mu.Lock()
	svc.startErr = nil
	svc.mu.Unlock()

	waitFor(t, func() bool { return statusOf(g, "microphones").State == StateRunning })

	if _, runs, _ := svc.counts(); runs != 1 {
		t.Errorf("Run was called %d times, want 1: it must not run before it has the device", runs)
	}

	cancel()
	<-done
}

// A service that is not supervised keeps the old behaviour: asked for once, and the group carries
// on without it. Something genuinely absent must not be retried forever.
func TestAServiceWithNoRestartIsAskedForOnce(t *testing.T) {
	svc := &fake{name: "camera", startErr: errors.New("no such device")}

	g := New()
	g.Add(svc, Once())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- g.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)

	starts, runs, _ := svc.counts()
	if starts != 1 {
		t.Errorf("asked for the device %d times, want 1", starts)
	}
	if runs != 0 {
		t.Errorf("Run was called %d times for a device it never got", runs)
	}

	cancel()
	<-done
}

// A service the process cannot work without still brings the group down rather than being retried.
func TestARequiredServiceStillGivesUp(t *testing.T) {
	svc := &fake{name: "display", startErr: errors.New("no panel")}

	g := New()
	g.Add(svc, Required(), Restart(time.Millisecond, 5*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := g.Run(ctx); err == nil {
		t.Fatal("a required service that could not be acquired did not stop the group")
	}
}
