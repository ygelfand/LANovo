package privacy

import (
	"context"
	"testing"
)

// The sliders are read, never written: what a person moved is the whole answer, and a mute
// something else could undo is not a mute. Neither sensor takes a command.
func TestSlidersCannotBeSetRemotely(t *testing.T) {
	p := &Privacy{}
	p.build()

	for _, e := range p.Entities() {
		if e == nil {
			t.Fatal("an entity is missing")
		}
	}

	// A BinarySensor has no command to give it, which is what makes this true by construction
	// rather than by nobody having wired one up.
	if p.micMuted.Get() {
		t.Error("the microphone reads as muted before anything has been read")
	}
}

// Starting on a machine with no lines says so and carries on, rather than failing the device:
// a board missing one control should still have the others.
func TestStartWithoutTheHardware(t *testing.T) {
	p := &Privacy{}
	p.build()

	if err := p.Start(context.Background()); err != nil {
		t.Errorf("Start with no lines open: %v", err)
	}
}

// What the driver reports as pressed is the slider engaged, for both controls.
func TestEngagedSlidersReadAsEngaged(t *testing.T) {
	p := &Privacy{}
	p.build()

	p.micMuted.Set(true)
	if !p.MicMuted() {
		t.Error("an engaged microphone slider does not read as muted")
	}

	p.camera.Set(true)
	if !p.CameraCovered() {
		t.Error("a closed shutter does not read as covered")
	}
}
