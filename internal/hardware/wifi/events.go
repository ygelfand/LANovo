package wifi

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The events worth acting on. wpa_supplicant sends many more.
const (
	EventStateChange  = "CTRL-EVENT-STATE-CHANGE"
	EventDisconnected = "CTRL-EVENT-DISCONNECTED"
	EventTerminating  = "CTRL-EVENT-TERMINATING"
)

// stateCompleted is wpa_state COMPLETED, as the state change reports it by number.
const stateCompleted = 9

// State is what a CTRL-EVENT-STATE-CHANGE carries: everything STATUS would have answered about
// the association, without asking.
//
// Asking is what caused the trouble. Answering STATUS makes the supplicant look its own address up
// and pass back through set_state, which emits this event again — so a STATUS sent from a handler
// for it is a loop that runs until something else stops it.
type State struct {
	Completed bool
	SSID      string
	BSSID     string
}

// state reads the fields off a state change. The text is "id=0 state=9 BSSID=.. SSID=..", and an
// SSID with a space in it is the rest of the line.
func state(text string) (State, bool) {
	rest, ok := strings.CutPrefix(text, EventStateChange+" ")
	if !ok {
		return State{}, false
	}

	var s State
	for rest != "" {
		var field string
		field, rest, _ = strings.Cut(rest, " ")

		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}

		switch key {
		case "state":
			n, err := strconv.Atoi(value)
			if err != nil {
				return State{}, false
			}
			s.Completed = n == stateCompleted
		case "BSSID":
			s.BSSID = value
		case "SSID":
			// Last, and may hold spaces, so it is the remainder rather than the field.
			s.SSID = value
			if rest != "" {
				s.SSID += " " + rest
			}
			rest = ""
		}
	}
	return s, true
}

// Event is an unsolicited message from the supplicant: a priority, and the text after it.
type Event struct {
	Priority int
	Text     string
}

// Is reports whether the event is of a kind, which is the first word of the text.
func (e Event) Is(kind string) bool { return strings.HasPrefix(e.Text, kind) }

// Listen calls handler for every event until ctx is canceled or the socket fails.
//
// On its own connection, so events never interleave with the replies to commands sent elsewhere.
// The approach is from github.com/hdiniz/wpa_supplicant-go (MIT).
func Listen(ctx context.Context, handler func(Event)) error {
	c, err := Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if out, err := c.Cmd("ATTACH"); err != nil {
		return err
	} else if !strings.HasPrefix(out, "OK") {
		return fmt.Errorf("wifi: ATTACH said %q", out)
	}
	defer c.Cmd("DETACH")

	buf := make([]byte, 4096)
	for {
		if ctx.Err() != nil {
			return nil
		}

		// A deadline rather than a blocking read, so canceling does not wait for an event that
		// may never come.
		if err := c.conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			return err
		}

		n, err := c.conn.Read(buf)
		if err != nil {
			if isTimeout(err) {
				continue
			}
			return fmt.Errorf("wifi: reading events: %w", err)
		}
		if e, ok := parseEvent(string(buf[:n])); ok {
			handler(e)
		}
	}
}

// parseEvent reads the "<3>CTRL-EVENT-CONNECTED ..." form.
func parseEvent(raw string) (Event, bool) {
	raw = strings.TrimRight(raw, "\n")
	if !strings.HasPrefix(raw, "<") {
		return Event{}, false
	}
	end := strings.Index(raw, ">")
	if end < 0 {
		return Event{}, false
	}

	priority, err := strconv.Atoi(raw[1:end])
	if err != nil {
		return Event{}, false
	}
	return Event{Priority: priority, Text: raw[end+1:]}, true
}

func isTimeout(err error) bool {
	type timeout interface{ Timeout() bool }
	t, ok := err.(timeout)
	return ok && t.Timeout()
}
