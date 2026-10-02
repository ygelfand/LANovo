package avrcp

import "testing"

// The multi-player events are registered for when the phone offers them. They are the only thing on
// the channel that says the metadata now belongs to something else.
func TestTheMultiPlayerEventsAreRegisteredFor(t *testing.T) {
	c := New()
	out, _ := c.Start()

	answers, err := c.Receive(respond(out[0], Stable, capabilities(
		EventPlaybackStatus, EventTrackChanged, EventPosition,
		EventSettings, EventNowPlaying, EventAvailPlayers, EventAddressPlayer, EventUIDs)))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}

	got := map[byte]bool{}
	for _, a := range answers {
		got[pduOf(t, a).Params[0]] = true
	}

	for _, want := range []byte{EventAddressPlayer, EventNowPlaying} {
		if !got[want] {
			t.Errorf("event %#02x was offered and not registered for", want)
		}
	}
}

// A different addressed player means what is on the screen belongs to whatever held it before, so
// ask again rather than keep showing it.
func TestADifferentAddressedPlayerAsksAgain(t *testing.T) {
	c := New()
	out, _ := c.Start()
	answers, _ := c.Receive(respond(out[0], Stable, capabilities(EventAddressPlayer)))

	registration := answers[0]

	// Interim: the player as it stands.
	first := PDU{ID: PDURegisterNotification, Params: []byte{EventAddressPlayer, 0x00, 0x01, 0x00, 0x00}}
	if _, err := c.Receive(respond(registration, Interim, first)); err != nil {
		t.Fatalf("the interim: %v", err)
	}

	moved := PDU{ID: PDURegisterNotification, Params: []byte{EventAddressPlayer, 0x00, 0x02, 0x00, 0x00}}
	sent, err := c.Receive(respond(registration, Changed, moved))
	if err != nil {
		t.Fatalf("the change: %v", err)
	}

	asked := map[byte]bool{}
	for _, s := range sent {
		asked[pduOf(t, s).ID] = true
	}

	if !asked[PDUGetElementAttributes] {
		t.Error("the player moved and nothing asked what is playing now")
	}
	if !asked[PDURegisterNotification] {
		t.Error("the registration was spent and not renewed")
	}
}

// The same player again is the far end restating where things stand, not a move.
func TestTheSameAddressedPlayerAsksNothing(t *testing.T) {
	c := New()
	out, _ := c.Start()
	answers, _ := c.Receive(respond(out[0], Stable, capabilities(EventAddressPlayer)))

	registration := answers[0]
	same := PDU{ID: PDURegisterNotification, Params: []byte{EventAddressPlayer, 0x00, 0x07, 0x00, 0x00}}

	if _, err := c.Receive(respond(registration, Interim, same)); err != nil {
		t.Fatalf("the interim: %v", err)
	}

	sent, err := c.Receive(respond(registration, Changed, same))
	if err != nil {
		t.Fatalf("the change: %v", err)
	}

	for _, s := range sent {
		if id := pduOf(t, s).ID; id == PDUGetElementAttributes {
			t.Error("the same player asked what is playing all over again")
		}
	}
}

// A rejection carries an error code where a completion carries the event, and the addressed player
// moving is reported that way against every registration outstanding at the time.
//
// Read as an event it marks some unrelated number unsupported and leaves the real one looking
// registered, which is a controller that stops hearing about the thing it asked for and never
// notices.
func TestAPlayerMovedRejectionRenewsTheRegistration(t *testing.T) {
	c := New()
	out, _ := c.Start()
	answers, _ := c.Receive(respond(out[0], Stable, capabilities(EventPlaybackStatus)))

	registration := answers[0]
	if got := pduOf(t, registration).Params[0]; got != EventPlaybackStatus {
		t.Fatalf("registered for %#02x", got)
	}

	rejected := PDU{ID: PDURegisterNotification, Params: []byte{RefusedPlayerMoved}}
	again, err := c.Receive(respond(registration, Rejected, rejected))
	if err != nil {
		t.Fatalf("the rejection: %v", err)
	}

	if len(again) != 1 {
		t.Fatalf("%d commands after the player moved, want the registration again", len(again))
	}

	p := pduOf(t, again[0])
	if p.ID != PDURegisterNotification {
		t.Fatalf("sent %#02x, want a registration", p.ID)
	}
	if p.Params[0] != EventPlaybackStatus {
		t.Errorf("renewed %#02x, want %#02x", p.Params[0], EventPlaybackStatus)
	}
}

// A real refusal is not retried. Asking forever is how a phone that does not report positions gets
// asked about them for the life of the connection.
func TestARealRefusalIsNotRetried(t *testing.T) {
	c := New()
	out, _ := c.Start()
	answers, _ := c.Receive(respond(out[0], Stable, capabilities(EventPlaybackStatus, EventPosition)))

	var position Transport
	for _, a := range answers {
		if pduOf(t, a).Params[0] == EventPosition {
			position = a
		}
	}
	if position.Payload == nil {
		t.Fatal("position was never registered for")
	}

	rejected := PDU{ID: PDURegisterNotification, Params: []byte{RefusedBadParameter}}
	again, err := c.Receive(respond(position, Rejected, rejected))
	if err != nil {
		t.Fatalf("the rejection: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("%d commands after a refusal, want none", len(again))
	}

	// And the one that was not refused is still live.
	if !c.supported[EventPlaybackStatus] {
		t.Error("refusing position also gave up on the playback status")
	}
	if c.supported[EventPosition] {
		t.Error("position was refused and is still thought supported")
	}
}

// A rejection names the event by its label. Two outstanding registrations refused at once must each
// renew their own.
func TestEachRejectionRenewsItsOwnEvent(t *testing.T) {
	c := New()
	out, _ := c.Start()
	answers, _ := c.Receive(respond(out[0], Stable,
		capabilities(EventPlaybackStatus, EventTrackChanged)))

	if len(answers) != 2 {
		t.Fatalf("%d registrations", len(answers))
	}

	rejected := PDU{ID: PDURegisterNotification, Params: []byte{RefusedPlayerMoved}}

	for _, was := range answers {
		want := pduOf(t, was).Params[0]

		again, err := c.Receive(respond(was, Rejected, rejected))
		if err != nil {
			t.Fatalf("the rejection for %#02x: %v", want, err)
		}
		if len(again) != 1 {
			t.Fatalf("%#02x drew %d renewals, want one", want, len(again))
		}
		if got := pduOf(t, again[0]).Params[0]; got != want {
			t.Errorf("%#02x was renewed as %#02x", want, got)
		}
	}
}
