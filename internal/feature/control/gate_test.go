package control

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
)

func TestOpenOnAFreshConfig(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))

	if !config.Defaults().Access.Control {
		t.Error("the control socket is off by default")
	}
	if !Enabled() {
		t.Error("Enabled says no on a fresh config")
	}
}

// Run must hold no socket while it is off, and must not treat that as a failure.
func TestRunHoldsNothingWhenOff(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	if err := config.Set().Access().Control(false); err != nil {
		t.Fatal(err)
	}

	c := build()
	c.addr = filepath.Join(t.TempDir(), "sock")
	ctx, stop := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	// Long enough that a Run which was going to listen has done so.
	time.Sleep(100 * time.Millisecond)

	if l, err := net.Listen("unix", c.addr); err != nil {
		t.Fatalf("the address is taken while the switch is off: %v", err)
	} else {
		l.Close()
	}

	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return when the context ended")
	}
}

// And the switch has to reach the socket without a restart.
func TestSwitchOpensAndCloses(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))

	c := build()
	c.addr = filepath.Join(t.TempDir(), "sock")
	ctx, stop := context.WithCancel(t.Context())
	defer stop()

	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	c.SetControl(true)
	if !config.Get().Access.Control {
		t.Error("turning it on was not saved")
	}
	if !waitFor(t, c.addr, true) {
		t.Fatal("the socket is not listening after the switch went on")
	}

	c.SetControl(false)
	if config.Get().Access.Control {
		t.Error("turning it off was not saved")
	}
	if !waitFor(t, c.addr, false) {
		t.Error("the socket is still listening after the switch went off")
	}

	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return when the context ended")
	}
}

// waitFor waits for the address to be taken, or to be free, and says whether it got there.
//
// Waiting in both directions rather than looking once: the switch signals the loop rather than
// doing the work itself, so the socket appears and disappears shortly after the call rather than
// during it, and a single look is a race in whichever direction it is not given.
func waitFor(t *testing.T, addr string, taken bool) bool {
	t.Helper()

	for range 40 {
		l, err := net.Listen("unix", addr)
		if err == nil {
			l.Close()
		}
		if (err != nil) == taken {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}
