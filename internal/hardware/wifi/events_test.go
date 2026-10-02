package wifi

import "testing"

func TestParseEvent(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		wantPriority int
		wantText     string
		wantOK       bool
	}{
		{
			name:         "connected",
			raw:          "<3>CTRL-EVENT-CONNECTED - Connection to f0:9f:c2:f5:50:5f completed\n",
			wantPriority: 3,
			wantText:     "CTRL-EVENT-CONNECTED - Connection to f0:9f:c2:f5:50:5f completed",
			wantOK:       true,
		},
		{
			name:         "terminating",
			raw:          "<2>CTRL-EVENT-TERMINATING",
			wantPriority: 2,
			wantText:     "CTRL-EVENT-TERMINATING",
			wantOK:       true,
		},
		{
			name:   "a reply to a command is not an event",
			raw:    "OK\n",
			wantOK: false,
		},
		{
			name:   "an unterminated priority is not an event",
			raw:    "<3 CTRL-EVENT-CONNECTED\n",
			wantOK: false,
		},
		{
			name:   "a priority that is not a number is not an event",
			raw:    "<x>CTRL-EVENT-CONNECTED\n",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseEvent(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("parseEvent(%q) ok = %v, want %v", tt.raw, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Priority != tt.wantPriority {
				t.Errorf("priority = %d, want %d", got.Priority, tt.wantPriority)
			}
			if got.Text != tt.wantText {
				t.Errorf("text = %q, want %q", got.Text, tt.wantText)
			}
		})
	}
}

func TestEventIs(t *testing.T) {
	e, ok := parseEvent("<3>CTRL-EVENT-STATE-CHANGE id=0 state=9 BSSID=f0:9f:c2:f5:50:5f SSID=attic\n")
	if !ok {
		t.Fatal("parseEvent rejected a state change")
	}

	if !e.Is(EventStateChange) {
		t.Error("a state change should be recognized as one")
	}
	if e.Is(EventDisconnected) {
		t.Error("a state change should not be a disconnection")
	}
}

// Everything settle used to ask STATUS for is already on the event. Asking made the supplicant
// emit the event again, and the two processes chased each other at 150% of a CPU.
func TestStateCarriesWhatStatusWouldHave(t *testing.T) {
	tests := []struct {
		name string
		text string
		want State
	}{
		{
			"associated",
			"CTRL-EVENT-STATE-CHANGE id=0 state=9 BSSID=f0:9f:c2:f5:50:5f SSID=iron.curtain",
			State{Completed: true, BSSID: "f0:9f:c2:f5:50:5f", SSID: "iron.curtain"},
		},
		{
			"still working on it",
			"CTRL-EVENT-STATE-CHANGE id=0 state=7 BSSID=f0:9f:c2:f5:50:5f SSID=iron.curtain",
			State{Completed: false, BSSID: "f0:9f:c2:f5:50:5f", SSID: "iron.curtain"},
		},
		{
			"a network named with spaces",
			"CTRL-EVENT-STATE-CHANGE id=0 state=9 BSSID=f0:9f:c2:f5:50:5f SSID=the back room",
			State{Completed: true, BSSID: "f0:9f:c2:f5:50:5f", SSID: "the back room"},
		},
		{
			"disconnected, which names no network",
			"CTRL-EVENT-STATE-CHANGE id=-1 state=0",
			State{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := state(tt.text)
			if !ok {
				t.Fatalf("state(%q) was not read", tt.text)
			}
			if got != tt.want {
				t.Errorf("state = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSomethingElseIsNotAStateChange(t *testing.T) {
	if _, ok := state("CTRL-EVENT-CONNECTED - connection to f0:9f:c2:f5:50:5f completed"); ok {
		t.Error("a connected event was read as a state change")
	}
}

// A state change with a state nobody can parse is dropped rather than read as a disconnection,
// which would take the network down over a message we did not understand.
func TestAnUnreadableStateIsNotADisconnection(t *testing.T) {
	if _, ok := state("CTRL-EVENT-STATE-CHANGE id=0 state=banana SSID=attic"); ok {
		t.Error("a state that is not a number was accepted")
	}
}
