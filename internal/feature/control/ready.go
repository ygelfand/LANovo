package control

import (
	"fmt"
	"strconv"
	"time"

	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

// How long to wait for the device to finish starting, and how often to look.
const (
	readyWait  = 90 * time.Second
	readyEvery = 200 * time.Millisecond
)

// ready blocks until the boot screen has let go of the panel.
//
// The control socket answers long before the device has finished starting, so waiting for the
// socket is waiting for the wrong thing: a whole test sequence has run against the splash and
// looked like a bug in what it was testing. This is the question that was actually being asked.
func ready(args []string) (string, error) {
	wait := readyWait
	if len(args) > 0 {
		ms, err := strconv.Atoi(args[0])
		if err != nil || ms < 0 {
			return "", fmt.Errorf("milliseconds must be a number")
		}
		wait = time.Duration(ms) * time.Millisecond
	}

	deadline := time.Now().Add(wait)
	for display.Get().Held(display.PriorityBoot) {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("still starting after %s", wait)
		}
		time.Sleep(readyEvery)
	}
	if shell.Get().Open() {
		return "screen", nil
	}
	return "dashboard", nil
}

// hold presses and keeps a finger down, then lifts it.
//
// A swipe from a point to itself does this already, by accident, and testing press feedback with
// one is a trick rather than a statement. Worth being able to say.
func hold(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("want X Y MILLISECONDS")
	}

	x, y, err := point(args)
	if err != nil {
		return err
	}

	ms, err := strconv.Atoi(args[2])
	if err != nil || ms < 0 {
		return fmt.Errorf("milliseconds must be a number")
	}

	return Get().press(x, y, time.Duration(ms)*time.Millisecond)
}
