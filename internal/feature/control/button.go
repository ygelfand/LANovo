package control

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/buttons"
)

// between is how long to leave between presses of a repeated one, which is about as fast as a
// thumb manages.
const between = 120 * time.Millisecond

// press is a button on the side of the device, as if somebody had pressed it.
//
//	button up|down|mute|... [times]
//
// The one control on this device that no amount of tapping the screen reaches, and the reason the
// volume card exists at all. Without it the card could only be raised by setting a level to
// something it was not, which picks the stream itself and so cannot test the thing that matters:
// that a press moves whichever level the card is showing.
//
// Delivered through the driver's own hook, so everything downstream — the card, the beep, Home
// Assistant — happens exactly as it does for a real press.
//
// Which means it makes a noise. Adjust chirps on purpose, because a press wants an answer at the
// new level, and there is no quiet version of this command for the same reason: one that skipped
// the chime would not be testing what a press does. Silence it at the source instead —
// `ctl volume feedback 0` — and put it back after.
func press(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("want %s", names())
	}

	button, ok := named(args[0])
	if !ok {
		return fmt.Errorf("no such button %q, want %s", args[0], names())
	}

	times := 1
	if len(args) > 1 {
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1 {
			return fmt.Errorf("times must be a positive number")
		}
		times = n
	}

	for i := range times {
		if i > 0 {
			time.Sleep(between)
		}
		buttons.Get().Deliver(buttons.Event{Button: button, Pressed: true})
		buttons.Get().Deliver(buttons.Event{Button: button, Pressed: false})
	}
	return nil
}

// named is the button somebody meant. Short names, because these are typed: "up" rather than
// "volume up", which would need quoting.
func named(s string) (buttons.Button, bool) {
	switch s {
	case "up":
		return buttons.VolumeUp, true
	case "down":
		return buttons.VolumeDown, true
	}

	// Anything else the driver knows, by its own name, so a button added there is reachable here
	// without being listed twice.
	for _, b := range buttons.All() {
		if string(b) == s {
			return b, true
		}
	}
	return "", false
}

func names() string {
	out := make([]string, 0, len(buttons.All())+2)
	out = append(out, "up", "down")

	for _, b := range buttons.All() {
		out = append(out, string(b))
	}
	return strings.Join(out, ", ")
}
