package wpa

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// fake is a supplicant that answers the handful of commands a switch sends, and remembers what it
// was asked. The interesting behaviour is which network it says it is on afterwards.
type fake struct {
	// on is what STATUS reports, and what select_network changes when the join is going to work.
	on string

	// joins is what a network with this name does when selected: true associates, false does not,
	// which is a wrong passphrase as far as anything here can tell.
	joins map[string]bool

	// configured is name by id, so remove_network and list_networks agree with each other.
	configured map[string]string
	next       int

	said  []string
	saved int
}

func newFake(on string) *fake {
	return &fake{
		on:         on,
		joins:      map[string]bool{},
		configured: map[string]string{},
	}
}

func (f *fake) Cmd(words ...string) (string, error) {
	f.said = append(f.said, strings.Join(words, " "))

	switch words[0] {
	case "status":
		if f.on == "" {
			return "wpa_state=DISCONNECTED", nil
		}
		return fmt.Sprintf("ssid=%s\nwpa_state=COMPLETED\nip_address=192.168.1.10", f.on), nil

	case "list_networks":
		out := "network id / ssid / bssid / flags"
		for id, name := range f.configured {
			out += fmt.Sprintf("\n%s\t%s\tany\t[CURRENT]", id, name)
		}
		return out, nil

	case "add_network":
		id := fmt.Sprint(f.next)
		f.next++
		f.configured[id] = ""
		return id, nil

	case "set_network":
		// set_network ID ssid HEX is the one that names it.
		if len(words) == 4 && words[2] == "ssid" {
			f.configured[words[1]] = unhex(words[3])
		}
		return "OK", nil

	case "remove_network":
		delete(f.configured, words[1])
		return "OK", nil

	case "select_network":
		name := f.configured[words[1]]
		if f.joins[name] {
			f.on = name
		}
		return "OK", nil

	case "enable_network", "reassociate", "disconnect":
		return "OK", nil

	case "save_config":
		f.saved++
		return "OK", nil
	}
	return "OK", nil
}

func (f *fake) asked(cmd string) bool {
	for _, s := range f.said {
		if strings.HasPrefix(s, cmd) {
			return true
		}
	}
	return false
}

func unhex(s string) string {
	var out []byte
	for i := 0; i+1 < len(s); i += 2 {
		var b int
		fmt.Sscanf(s[i:i+2], "%02x", &b)
		out = append(out, byte(b))
	}
	return string(out)
}

func TestSwitchingToAWorkingNetworkTakes(t *testing.T) {
	f := newFake("old")
	f.joins["new"] = true

	if err := Switch(context.Background(), f, "new", "a good passphrase"); err != nil {
		t.Fatalf("switching to a network that associates: %v", err)
	}
	if f.on != "new" {
		t.Errorf("on %q after switching, want %q", f.on, "new")
	}
	if f.saved == 0 {
		t.Error("the configuration was not saved, so the network would not survive a reboot")
	}
}

// The whole point of the task: a wrong passphrase must not strand the device.
func TestAFailedSwitchGoesBack(t *testing.T) {
	f := newFake("old")
	f.joins["new"] = false

	within, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()

	err := Switch(within, f, "new", "the wrong passphrase")
	if err == nil {
		t.Fatal("switching to a network that never associates reported success")
	}
	if !strings.Contains(err.Error(), "old") {
		t.Errorf("the error is %q, want it to say which network it went back to", err)
	}

	if f.on != "old" {
		t.Errorf("on %q after a failed switch, want to be back on %q", f.on, "old")
	}
	if !f.asked("enable_network all") || !f.asked("reassociate") {
		t.Error("nothing put the other networks back: select_network disables them all")
	}
}

// A network that did not associate must not be left configured, or every reboot retries it.
func TestAFailedSwitchLeavesNothingBehind(t *testing.T) {
	f := newFake("old")
	f.joins["new"] = false

	within, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	_ = Switch(within, f, "new", "the wrong passphrase")

	for id, name := range f.configured {
		if name == "new" {
			t.Errorf("network %s is still configured as %q after failing to join", id, name)
		}
	}
}

// The configuration on disk never had the new network in it, so a failure has nothing to write.
func TestAFailedSwitchDoesNotSave(t *testing.T) {
	f := newFake("old")
	f.joins["new"] = false

	within, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	_ = Switch(within, f, "new", "the wrong passphrase")

	if f.saved != 0 {
		t.Errorf("saved the configuration %d times after a failed switch, want none", f.saved)
	}
}

func TestSwitchingToTheNetworkAlreadyOnIsRefused(t *testing.T) {
	f := newFake("old")

	if err := Switch(context.Background(), f, "old", "whatever"); err == nil {
		t.Error("switching to the network already in use reported success")
	}
	if f.asked("add_network") {
		t.Error("it configured a second copy of the network it was already on")
	}
}

func TestSwitchingToNothingIsRefused(t *testing.T) {
	f := newFake("old")

	if err := Switch(context.Background(), f, "", "whatever"); err == nil {
		t.Error("switching to an unnamed network reported success")
	}
}

// A device on no network has nothing to go back to, which is not a reason to hang on to a network
// that does not work.
func TestAFailedSwitchFromNothingStillCleansUp(t *testing.T) {
	f := newFake("")
	f.joins["new"] = false

	within, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()

	if err := Switch(within, f, "new", "the wrong passphrase"); err == nil {
		t.Fatal("switching to a network that never associates reported success")
	}
	if len(f.configured) != 0 {
		t.Errorf("%d networks left configured, want none", len(f.configured))
	}
}

// A bad passphrase is caught before anything is torn down, so the device never leaves the network
// it is on.
func TestABadPassphraseNeverTouchesTheConnection(t *testing.T) {
	f := newFake("old")

	if err := Switch(context.Background(), f, "new", "short"); err == nil {
		t.Fatal("a five character passphrase was accepted")
	}
	if f.on != "old" {
		t.Errorf("on %q, want to still be on %q", f.on, "old")
	}
	if f.asked("select_network") {
		t.Error("it selected a network before the passphrase was checked")
	}
}
