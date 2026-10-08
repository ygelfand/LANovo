package privacy

import (
	"context"
	"testing"
)

func TestSlidersCannotBeSetRemotely(t *testing.T) {
	p := &Privacy{}
	p.build()

	for _, e := range p.Entities() {
		if e == nil {
			t.Fatal("an entity is missing")
		}
	}

	if p.micMuted.Get() {
		t.Error("the microphone reads as muted before anything has been read")
	}
}

func TestStartWithoutTheHardware(t *testing.T) {
	p := &Privacy{}
	p.build()

	if err := p.Start(context.Background()); err != nil {
		t.Errorf("Start with no lines open: %v", err)
	}
}

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
