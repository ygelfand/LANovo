package sendspin

import "testing"

// Losing the speaker pauses the group rather than muting the room.
//
// Muting left the server sending to nobody and the audio coming back by itself the moment the
// speaker was free, which is what happened going group, then a phone, then the group again.

func TestLosingTheSpeakerPausesTheGroup(t *testing.T) {
	paused := 0

	o := &out{gain: 1}
	o.pause = func() { paused++ }

	o.Stand(true)
	if paused != 1 {
		t.Fatalf("standing down asked for %d pauses, want one", paused)
	}
	if !o.held {
		t.Error("standing down did not hold the output")
	}
}

// The arbiter delivers a standing again whenever anything moves, so the same one twice is once.
func TestTheSameStandingDoesNotPauseAgain(t *testing.T) {
	paused := 0

	o := &out{gain: 1}
	o.pause = func() { paused++ }

	o.Stand(true)
	o.Stand(true)
	o.Stand(true)

	if paused != 1 {
		t.Errorf("a repeated standing asked for %d pauses, want one", paused)
	}
}

// Getting the speaker back says nothing to the group. A room that was interrupted stays paused
// until somebody presses play.
func TestGettingTheSpeakerBackSaysNothing(t *testing.T) {
	paused := 0

	o := &out{gain: 1}
	o.pause = func() { paused++ }

	o.Stand(true)
	o.Stand(false)

	if paused != 1 {
		t.Errorf("%d pauses across a standing and a release, want one", paused)
	}
	if o.held {
		t.Error("standing back up left the output held")
	}
}

// Nothing is registered before a group is joined, and standing must not reach for it.
func TestStandingWithNobodyToTellIsSafe(t *testing.T) {
	o := &out{gain: 1}

	o.Stand(true)
	o.Stand(false)
}
