package chromecast

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

// Nothing here turns the receiver on. Doing so binds a port and puts a service on the network,
// which is not something a test should do to whatever machine it runs on — and is also the thing
// the switch exists to stop happening by accident.

func fresh(t *testing.T) *Receiver {
	t.Helper()

	config.Use(filepath.Join(t.TempDir(), "state.json"))
	return build()
}

// A device that advertises itself on the network is not something to turn on for somebody.
func TestTheReceiverIsOffUntilAsked(t *testing.T) {
	r := fresh(t)

	if r.Enabled() {
		t.Error("a device nobody has configured is already casting")
	}
	if config.Defaults().Cast.Receiver {
		t.Error("the default is on")
	}
}

// Off, it holds nothing: no socket, no service, nothing to take down.
func TestStartingWhileOffDoesNothing(t *testing.T) {
	r := fresh(t)

	if err := r.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if r.listener != nil {
		t.Error("a socket was opened")
	}
	if r.advert != nil {
		t.Error("a service was advertised")
	}
	if r.Playing() != nil {
		t.Error("something is playing")
	}
}

// Closing something that never started is what happens on every restart where the switch was off.
func TestClosingWithoutStartingIsSafe(t *testing.T) {
	r := fresh(t)

	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("closing twice: %v", err)
	}
}

// The switch is what Home Assistant and the harness both come through, so it has to carry the
// setting rather than shadow it.
func TestTheSwitchFollowsTheSetting(t *testing.T) {
	r := fresh(t)

	if r.enable.Get() {
		t.Error("the switch reads on for a receiver that is off")
	}

	// Turning it off when it is already off saves the setting and starts nothing.
	r.SetReceiver(false)

	if config.Get().Cast.Receiver {
		t.Error("the setting says on")
	}
	if r.enable.Get() {
		t.Error("the switch says on")
	}
	if r.listener != nil {
		t.Error("turning it off opened a socket")
	}
}

// The entity has to be there for Home Assistant to show anything at all, and its identifier is what
// an automation refers to — renaming it silently breaks whatever was pointing at it.
func TestTheSwitchIsPublished(t *testing.T) {
	r := fresh(t)

	var ids []string
	for _, e := range r.Entities() {
		ids = append(ids, e.Object())
	}
	want := []string{"cast_receiver", "cast_oracle", "cast_credentials", "cast_credentials_expire", "youtube_lounge_on_demand", "youtube_sponsorblock", "youtube_live_delay"}
	if !slices.Equal(ids, want) {
		t.Errorf("the entities are %v, want %v", ids, want)
	}
}

func TestTheComponentIsNamed(t *testing.T) {
	if got := fresh(t).Name(); got == "" {
		t.Error("the component has no name, so nothing in a log says which one it is")
	}
}
