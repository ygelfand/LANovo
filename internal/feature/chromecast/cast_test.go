package chromecast

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

func fresh(t *testing.T) *Receiver {
	t.Helper()

	config.Use(filepath.Join(t.TempDir(), "state.json"))
	return build()
}

func TestTheReceiverIsOffUntilAsked(t *testing.T) {
	r := fresh(t)

	if r.Enabled() {
		t.Error("a device nobody has configured is already casting")
	}
	if config.Defaults().Cast.Receiver {
		t.Error("the default is on")
	}
}

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

func TestClosingWithoutStartingIsSafe(t *testing.T) {
	r := fresh(t)

	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("closing twice: %v", err)
	}
}

func TestTheSwitchFollowsTheSetting(t *testing.T) {
	r := fresh(t)

	if r.enable.Get() {
		t.Error("the switch reads on for a receiver that is off")
	}

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

func TestTheSwitchIsPublished(t *testing.T) {
	r := fresh(t)

	var ids []string
	for _, e := range r.Entities() {
		ids = append(ids, e.Object())
	}
	want := []string{
		"cast_receiver",
		"cast_oracle",
		"cast_credentials",
		"cast_credentials_expire",
		"youtube_lounge_on_demand",
		"youtube_sponsorblock",
		"youtube_live_delay",
	}
	if !slices.Equal(ids, want) {
		t.Errorf("the entities are %v, want %v", ids, want)
	}
}

func TestTheComponentIsNamed(t *testing.T) {
	if got := fresh(t).Name(); got == "" {
		t.Error("the component has no name, so nothing in a log says which one it is")
	}
}
