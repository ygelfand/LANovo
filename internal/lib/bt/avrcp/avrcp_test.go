package avrcp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

func TestATransportMessageRoundTrips(t *testing.T) {
	want := Transport{Label: 0x0b, Type: MessageResponse, Payload: []byte{1, 2, 3}}

	got, err := ParseTransport(want.Marshal())
	if err != nil {
		t.Fatalf("ParseTransport: %v", err)
	}
	if got.Label != want.Label || got.Type != want.Type {
		t.Errorf("came back %+v", got)
	}
	if !bytes.Equal(got.Payload, want.Payload) {
		t.Errorf("payload %v", got.Payload)
	}
}

// Half an AV/C frame parses into a plausible command for the wrong thing, so a fragment has to be
// refused rather than read short.
func TestAFragmentIsNotReadAsAWholeMessage(t *testing.T) {
	buf := []byte{0x00 | PacketStart<<2, 0x02, 0x11, 0x0e}

	if _, err := ParseTransport(buf); err == nil {
		t.Error("a start packet was read as a whole message")
	}
}

func TestAMessageForAnotherProfileIsRefused(t *testing.T) {
	buf := []byte{0x00, 0x11, 0x1f}

	if _, err := ParseTransport(buf); err == nil {
		t.Error("another profile's message was accepted")
	}
}

func TestATransportMessageTooShortToBeOne(t *testing.T) {
	if _, err := ParseTransport([]byte{0x00, 0x11}); !errors.Is(err, ErrShort) {
		t.Errorf("gave %v, want a short read", err)
	}
}

// A metadata response with a long title does not fit one packet.
func TestAFragmentedMessageIsPutBackTogether(t *testing.T) {
	var r Reassembler

	// Start: label 3, two packets, the count byte, then the profile.
	start := []byte{0x30 | PacketStart<<2, 0x02, 0x11, 0x0e, 0xaa, 0xbb}
	if got, err := r.Push(start); err != nil || got != nil {
		t.Fatalf("the start packet gave %v, %v", got, err)
	}

	end := []byte{0x30 | PacketEnd<<2, 0x11, 0x0e, 0xcc}
	got, err := r.Push(end)
	if err != nil {
		t.Fatalf("the end packet: %v", err)
	}

	// Rebuilt as a single packet, so it parses like one.
	m, err := ParseTransport(got)
	if err != nil {
		t.Fatalf("the rebuilt message did not parse: %v", err)
	}
	if m.Label != 3 {
		t.Errorf("label %d, want 3", m.Label)
	}
	if !bytes.Equal(m.Payload, []byte{0xaa, 0xbb, 0xcc}) {
		t.Errorf("payload %v", m.Payload)
	}
}

func TestAWholeMessagePassesStraightThroughTheReassembler(t *testing.T) {
	var r Reassembler

	buf := []byte{0x00, 0x11, 0x0e, 0x42}
	got, err := r.Push(buf)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if !bytes.Equal(got, buf) {
		t.Errorf("came back %v", got)
	}
}

// A continuation for a run this side never saw the start of cannot be placed.
func TestAContinuationWithNoStartIsRefused(t *testing.T) {
	var r Reassembler

	if _, err := r.Push([]byte{0x30 | PacketEnd<<2, 0x11, 0x0e, 0x01}); err == nil {
		t.Error("an orphan end packet was accepted")
	}
}

// Two transactions interleaved. Joining one onto the other builds a message out of two answers.
func TestAContinuationForAnotherTransactionIsRefused(t *testing.T) {
	var r Reassembler

	r.Push([]byte{0x30 | PacketStart<<2, 0x02, 0x11, 0x0e, 0xaa})

	if _, err := r.Push([]byte{0x40 | PacketEnd<<2, 0x11, 0x0e, 0xbb}); err == nil {
		t.Error("a continuation for a different label was joined on")
	}
}

func TestARunThatEndsEarlyIsRefused(t *testing.T) {
	var r Reassembler

	r.Push([]byte{0x30 | PacketStart<<2, 0x04, 0x11, 0x0e, 0xaa})
	r.Push([]byte{0x30 | PacketContinue<<2, 0x11, 0x0e, 0xbb})

	if _, err := r.Push([]byte{0x30 | PacketEnd<<2, 0x11, 0x0e, 0xcc}); err == nil {
		t.Error("a run two packets short came back whole")
	}
}

func TestAnAVCFrameRoundTrips(t *testing.T) {
	want := AVC{Code: Control, Subunit: SubunitPanel, Opcode: OpPassThrough, Operands: []byte{1, 2}}

	got, err := ParseAVC(want.Marshal())
	if err != nil {
		t.Fatalf("ParseAVC: %v", err)
	}
	if got.Code != want.Code || got.Subunit != want.Subunit || got.Opcode != want.Opcode {
		t.Errorf("came back %+v", got)
	}
	if !bytes.Equal(got.Operands, want.Operands) {
		t.Errorf("operands %v", got.Operands)
	}
}

// A press with no release leaves the phone holding the button down, which on a seek runs to the
// end of the track.
func TestAButtonCarriesItsPressAndItsRelease(t *testing.T) {
	for _, released := range []bool{Pressed, Released} {
		frame := PassThrough(OpNext, released)

		op, back, err := Button(frame)
		if err != nil {
			t.Fatalf("Button: %v", err)
		}
		if op != OpNext {
			t.Errorf("button %#02x, want next", op)
		}
		if back != released {
			t.Errorf("released came back %v, want %v", back, released)
		}
	}
}

// The operation data length is not optional; most targets refuse a frame without it.
func TestAButtonCarriesItsOperandLength(t *testing.T) {
	if got := PassThrough(OpPlay, Pressed); len(got.Operands) != 2 || got.Operands[1] != 0 {
		t.Errorf("operands %v, want the button and a zero length", got.Operands)
	}
}

func TestAPDURoundTrips(t *testing.T) {
	want := PDU{ID: PDUGetPlayStatus, Params: []byte{1, 2, 3}}

	frame := want.Frame(Status)
	if frame.Opcode != OpVendorDependent {
		t.Fatalf("a pdu was wrapped in opcode %#02x", frame.Opcode)
	}

	got, err := ParsePDU(frame.Operands)
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("pdu %#02x, want %#02x", got.ID, want.ID)
	}
	if !bytes.Equal(got.Params, want.Params) {
		t.Errorf("params %v", got.Params)
	}
}

// Somebody else's vendor extension is not an AVRCP pdu, and reading it as one gets a pdu id out of
// whatever their encoding put in that byte.
func TestAnotherCompanysExtensionIsRefused(t *testing.T) {
	operands := []byte{0x00, 0x11, 0x22, PDUGetPlayStatus, 0x00, 0x00, 0x00}

	if _, err := ParsePDU(operands); err == nil {
		t.Error("another company's vendor frame was read as avrcp")
	}
}

// The length in the header is what says where the parameters end.
func TestAPDUShorterThanItsLengthSays(t *testing.T) {
	operands := []byte{0x00, 0x19, 0x58, PDUGetPlayStatus, 0x00, 0x00, 0x08, 0x01}

	if _, err := ParsePDU(operands); !errors.Is(err, ErrShort) {
		t.Errorf("gave %v, want a short read", err)
	}
}

func attribute(id uint32, value string) []byte {
	out := binary.BigEndian.AppendUint32(nil, id)
	out = binary.BigEndian.AppendUint16(out, UTF8)
	out = binary.BigEndian.AppendUint16(out, uint16(len(value)))
	return append(out, value...)
}

func elementAttributes(list ...[]byte) []byte {
	out := []byte{byte(len(list))}
	for _, a := range list {
		out = append(out, a...)
	}
	return out
}

func TestReadingWhatIsPlaying(t *testing.T) {
	params := elementAttributes(
		attribute(AttrTitle, "Sun Ra"),
		attribute(AttrArtist, "Arkestra"),
		attribute(AttrAlbum, "Space Is The Place"),
		attribute(AttrDuration, "215000"),
		attribute(AttrTrack, "3"),
	)

	got, err := ParseElementAttributes(params)
	if err != nil {
		t.Fatalf("ParseElementAttributes: %v", err)
	}
	if got.Title != "Sun Ra" || got.Artist != "Arkestra" {
		t.Errorf("came back %+v", got)
	}
	if got.Duration != 215*time.Second {
		t.Errorf("duration %v, want 3m35s", got.Duration)
	}
	if got.Number != "3" {
		t.Errorf("track number %q", got.Number)
	}
}

// An attribute cut off part way through is dropped and the ones before it are kept.
//
// Refusing the whole response instead throws away good fields over a bad one, and the count cannot
// be trusted to mean what it says: a phone was seen answering seven with nothing behind it. bluez
// reads what is there for the same reason.
func TestATruncatedAttributeIsDroppedNotTheWholeAnswer(t *testing.T) {
	params := elementAttributes(
		attribute(AttrTitle, "Sun Ra"),
		attribute(AttrArtist, "Arkestra"),
	)
	params = params[:len(params)-2]

	got, err := ParseElementAttributes(params)
	if err != nil {
		t.Fatalf("a truncated attribute gave %v, want the attributes before it", err)
	}
	if got.Title != "Sun Ra" {
		t.Errorf("the title before the truncated one is %q", got.Title)
	}
	if got.Artist != "" {
		t.Errorf("the truncated attribute came through as %q", got.Artist)
	}
}

// A count larger than the attributes actually sent, which is what a phone does.
func TestACountLargerThanWhatArrived(t *testing.T) {
	params := elementAttributes(attribute(AttrTitle, "Sun Ra"))
	params[0] = 4

	got, err := ParseElementAttributes(params)
	if err != nil {
		t.Fatalf("a count of four with one attribute gave %v", err)
	}
	if got.Title != "Sun Ra" {
		t.Errorf("the one attribute that arrived is %q", got.Title)
	}
}

// The case seen on the wire: a count and nothing behind it at all.
func TestACountWithNothingBehindIt(t *testing.T) {
	got, err := ParseElementAttributes([]byte{7})
	if err != nil {
		t.Fatalf("seven attributes and no data gave %v, want an empty track", err)
	}
	if got.Title != "" || got.Artist != "" {
		t.Errorf("came back with %+v", got)
	}
}

// A phone sending "--" for a live stream is not malformed, and refusing the response over it would
// lose the title too.
func TestADurationThatIsNotANumberIsJustUnknown(t *testing.T) {
	got, err := ParseElementAttributes(elementAttributes(
		attribute(AttrTitle, "Some Station"),
		attribute(AttrDuration, "--"),
	))
	if err != nil {
		t.Fatalf("ParseElementAttributes: %v", err)
	}
	if got.Duration != 0 {
		t.Errorf("duration %v, want none", got.Duration)
	}
	if got.Title != "Some Station" {
		t.Errorf("the title was lost: %q", got.Title)
	}
}

// A run of digits long enough to wrap would come back as a negative duration.
func TestARidiculousDurationIsNotTakenSeriously(t *testing.T) {
	got, _ := ParseElementAttributes(elementAttributes(
		attribute(AttrDuration, "99999999999999999999"),
	))
	if got.Duration != 0 {
		t.Errorf("duration %v, want none", got.Duration)
	}
}

func TestReadingWhereTheTrackHasGotTo(t *testing.T) {
	params := binary.BigEndian.AppendUint32(nil, 215000)
	params = binary.BigEndian.AppendUint32(params, 42000)
	params = append(params, StatusPlaying)

	got, err := ParsePlayStatus(params)
	if err != nil {
		t.Fatalf("ParsePlayStatus: %v", err)
	}
	if got.Length != 215*time.Second || got.Position != 42*time.Second {
		t.Errorf("came back %+v", got)
	}
	if got.Status != StatusPlaying {
		t.Errorf("status %#02x", got.Status)
	}
}

// 0xffffffff is the spec's "not known", which a stream sends. Forty-nine days on a progress bar is
// worse than nothing.
func TestAnUnknownLengthIsNotFortyNineDays(t *testing.T) {
	params := binary.BigEndian.AppendUint32(nil, 0xffffffff)
	params = binary.BigEndian.AppendUint32(params, 0xffffffff)
	params = append(params, StatusPlaying)

	got, err := ParsePlayStatus(params)
	if err != nil {
		t.Fatalf("ParsePlayStatus: %v", err)
	}
	if got.Length != 0 || got.Position != 0 {
		t.Errorf("came back %+v, want both unknown", got)
	}
}

func TestReadingANotification(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  []byte
		event byte
		check func(Notification) bool
	}{
		{"playback status", []byte{StatusPaused}, EventPlaybackStatus,
			func(n Notification) bool { return n.Status == StatusPaused }},
		{"track", make([]byte, 8), EventTrackChanged,
			func(n Notification) bool { return n.Track == 0 }},
		{"position", binary.BigEndian.AppendUint32(nil, 5000), EventPosition,
			func(n Notification) bool { return n.Position == 5*time.Second }},
		{"volume", []byte{0x40}, EventVolume,
			func(n Notification) bool { return n.Volume == 0x40 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseNotification(append([]byte{tc.event}, tc.body...), false)
			if err != nil {
				t.Fatalf("ParseNotification: %v", err)
			}
			if !tc.check(got) {
				t.Errorf("came back %+v", got)
			}
		})
	}
}

func TestATruncatedNotification(t *testing.T) {
	if _, err := ParseNotification([]byte{EventTrackChanged, 0x01}, false); !errors.Is(err, ErrShort) {
		t.Errorf("gave %v, want a short read", err)
	}
}

// Absolute volume is seven bits, not a hundred. The ends have to be exact or the slider cannot
// reach silence or full.
func TestVolumeConvertsAtBothEnds(t *testing.T) {
	if got := Volume(0); got != 0 {
		t.Errorf("zero percent is %d", got)
	}
	if got := Volume(100); got != VolumeMax {
		t.Errorf("a hundred percent is %d, want %d", got, VolumeMax)
	}
	if got := Volume(500); got != VolumeMax {
		t.Errorf("a volume over a hundred wrapped to %d", got)
	}
	if got := Volume(-5); got != 0 {
		t.Errorf("a negative volume came back %d", got)
	}

	if got := Percent(0); got != 0 {
		t.Errorf("zero came back %d%%", got)
	}
	if got := Percent(VolumeMax); got != 100 {
		t.Errorf("full came back %d%%", got)
	}
}

func TestVolumeRoundTripsWithinARoundingStep(t *testing.T) {
	for percent := 0; percent <= 100; percent++ {
		if got := Percent(Volume(percent)); got < percent-1 || got > percent+1 {
			t.Errorf("%d%% came back %d%%", percent, got)
		}
	}
}

// pduOf reads the pdu out of a command a controller produced.
func pduOf(t *testing.T, tr Transport) PDU {
	t.Helper()

	frame, err := ParseAVC(tr.Payload)
	if err != nil {
		t.Fatalf("ParseAVC: %v", err)
	}

	p, err := ParsePDU(frame.Operands)
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}
	return p
}

// respond builds the answer to a command, on its own label.
func respond(to Transport, code byte, p PDU) Transport {
	return Transport{Label: to.Label, Type: MessageResponse, Payload: p.Frame(code).Marshal()}
}

func capabilities(events ...byte) PDU {
	params := append([]byte{CapabilityEvents, byte(len(events))}, events...)
	return PDU{ID: PDUGetCapabilities, Params: params}
}

// Coming up: ask what the phone can report, then what is playing.
func TestStartingUpAsksWhatThePhoneCanDo(t *testing.T) {
	c := New()

	out, err := c.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("%d commands, want capabilities, attributes, the artwork and status", len(out))
	}

	want := []byte{
		PDUGetCapabilities, PDUGetElementAttributes, PDUGetElementAttributes, PDUGetPlayStatus,
	}
	for i, p := range out {
		if got := pduOf(t, p).ID; got != want[i] {
			t.Errorf("command %d is %#02x, want %#02x", i, got, want[i])
		}
	}

	// Every one on its own label, or the answers cannot be told apart.
	seen := map[byte]bool{}
	for _, p := range out {
		if seen[p.Label] {
			t.Errorf("label %d was used twice", p.Label)
		}
		seen[p.Label] = true
	}
}

// Registering for an event a phone does not support is refused every time it is asked, so the
// supported list is what decides.
func TestOnlySupportedEventsAreRegisteredFor(t *testing.T) {
	c := New()
	out, _ := c.Start()

	answers, err := c.Receive(respond(out[0], Stable, capabilities(EventPlaybackStatus, EventTrackChanged)))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if len(answers) != 2 {
		t.Fatalf("%d registrations, want one per supported event", len(answers))
	}

	for _, a := range answers {
		p := pduOf(t, a)
		if p.ID != PDURegisterNotification {
			t.Errorf("sent %#02x, want a registration", p.ID)
		}
		if p.Params[0] == EventPosition {
			t.Error("registered for position, which this phone did not offer")
		}
	}
}

// Position is the only event where the interval means anything.
func TestPositionIsRegisteredForWithAnInterval(t *testing.T) {
	c := New()
	out, _ := c.Start()

	answers, _ := c.Receive(respond(out[0], Stable, capabilities(EventPosition)))
	if len(answers) != 1 {
		t.Fatalf("%d registrations", len(answers))
	}

	p := pduOf(t, answers[0])
	if got := binary.BigEndian.Uint32(p.Params[1:]); got != 1 {
		t.Errorf("registered with an interval of %d, want one second", got)
	}
}

// A registration is one-shot. The interim answer is the current value; the one after it is the
// change, and after that the phone has forgotten — so it has to be made again or the screen
// updates exactly once.
func TestARegistrationIsRenewedAfterItFires(t *testing.T) {
	c := New()

	var states []State
	c.Changed = func(s State) { states = append(states, s) }

	out, _ := c.Start()
	registrations, _ := c.Receive(respond(out[0], Stable, capabilities(EventPlaybackStatus)))
	registration := registrations[0]

	// Interim: what it is now. Nothing to renew yet.
	interim := PDU{ID: PDURegisterNotification, Params: []byte{EventPlaybackStatus, StatusPaused}}
	again, err := c.Receive(respond(registration, Interim, interim))
	if err != nil {
		t.Fatalf("the interim answer: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("the interim answer produced %d commands, want none", len(again))
	}
	if c.State().Progress.Status != StatusPaused {
		t.Errorf("status is %#02x", c.State().Progress.Status)
	}

	// Changed: the registration is now spent.
	changed := PDU{ID: PDURegisterNotification, Params: []byte{EventPlaybackStatus, StatusPlaying}}
	again, err = c.Receive(respond(registration, Changed, changed))
	if err != nil {
		t.Fatalf("the changed answer: %v", err)
	}
	// Renewed, and asked what is playing: a source that reports the status and not the track gives
	// no other sign that there is something new to read.
	var renewed, attributes bool
	for _, sent := range again {
		switch p := pduOf(t, sent); p.ID {
		case PDURegisterNotification:
			renewed = renewed || p.Params[0] == EventPlaybackStatus
		case PDUGetElementAttributes:
			attributes = true
		}
	}

	if !renewed {
		t.Errorf("%d commands after the notification fired, none renewing the registration", len(again))
	}
	if !attributes {
		t.Error("playback started and nothing asked what is playing")
	}

	if c.State().Progress.Status != StatusPlaying {
		t.Errorf("status is %#02x, want playing", c.State().Progress.Status)
	}
	if len(states) == 0 {
		t.Error("nothing was published to the screen")
	}
}

// A track change says only that it changed, not to what.
func TestATrackChangeAsksWhatTheNewOneIs(t *testing.T) {
	c := New()
	out, _ := c.Start()
	registrations, _ := c.Receive(respond(out[0], Stable, capabilities(EventTrackChanged)))

	body := PDU{ID: PDURegisterNotification, Params: append([]byte{EventTrackChanged}, make([]byte, 8)...)}
	again, err := c.Receive(respond(registrations[0], Changed, body))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}

	var asked bool
	for _, a := range again {
		if pduOf(t, a).ID == PDUGetElementAttributes {
			asked = true
		}
	}
	if !asked {
		t.Error("a track change did not ask what the new track is")
	}
}

// Retrying a refused registration is how a phone gets asked forever about something it does not do.
func TestARefusedRegistrationIsNotRetried(t *testing.T) {
	c := New()
	out, _ := c.Start()
	registrations, _ := c.Receive(respond(out[0], Stable, capabilities(EventPosition)))

	body := PDU{ID: PDURegisterNotification, Params: []byte{EventPosition}}
	again, err := c.Receive(respond(registrations[0], Rejected, body))
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("a refused registration produced %d commands", len(again))
	}

	// And it is not offered again by a later round either.
	more, _ := c.register(EventPosition)
	if len(more) != 0 {
		t.Error("a refused event was registered for again")
	}
}

// A phone between tracks answers with everything blank. Taking it replaces the title on the screen
// with nothing.
func TestBlankMetadataDoesNotWipeTheScreen(t *testing.T) {
	c := New()

	out, _ := c.Start()
	c.Receive(respond(out[1], Stable, PDU{
		ID:     PDUGetElementAttributes,
		Params: elementAttributes(attribute(AttrTitle, "Sun Ra")),
	}))

	if c.State().Track.Title != "Sun Ra" {
		t.Fatalf("the title did not arrive: %q", c.State().Track.Title)
	}

	refresh, _ := c.Refresh()
	c.Receive(respond(refresh[0], Stable, PDU{
		ID:     PDUGetElementAttributes,
		Params: elementAttributes(attribute(AttrTitle, ""), attribute(AttrArtist, "")),
	}))

	if got := c.State().Track.Title; got != "Sun Ra" {
		t.Errorf("the title became %q, want the last one left up", got)
	}
}

// The phone's own volume slider moving this device is what a phone expects of anything it plays to.
func TestThePhoneCanSetTheVolume(t *testing.T) {
	c := New()

	var set int = -1
	c.Volume = func(percent int) { set = percent }

	command := Transport{
		Label:   5,
		Type:    MessageCommand,
		Payload: SetAbsoluteVolume(Volume(50)).Frame(Control).Marshal(),
	}

	out, err := c.Receive(command)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("%d answers, want one", len(out))
	}
	if out[0].Label != 5 {
		t.Errorf("answered on label %d, want the one asked on", out[0].Label)
	}
	if out[0].Type != MessageResponse {
		t.Error("the answer is not a response")
	}
	if set < 49 || set > 51 {
		t.Errorf("the volume was set to %d%%, want about 50", set)
	}
}

// A phone waiting on an answer retries until it gives up on the channel entirely.
func TestACommandThisDoesNotImplementIsStillAnswered(t *testing.T) {
	c := New()

	command := Transport{
		Label:   7,
		Type:    MessageCommand,
		Payload: AVC{Code: Control, Subunit: SubunitPanel, Opcode: OpSubunitInfo}.Marshal(),
	}

	out, err := c.Receive(command)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("%d answers, want one refusal", len(out))
	}

	frame, err := ParseAVC(out[0].Payload)
	if err != nil {
		t.Fatalf("ParseAVC: %v", err)
	}
	if frame.Code != NotImplemented {
		t.Errorf("answered %#02x, want not implemented", frame.Code)
	}
}

// Reusing a label that is outstanding means the next answer cannot be matched to its question, so
// running out is an error rather than handing one out twice.
func TestLabelsAreNotReusedWhileOutstanding(t *testing.T) {
	c := New()

	seen := map[byte]bool{}
	for i := range 16 {
		out, err := c.SetVolume(i)
		if err != nil {
			t.Fatalf("command %d of sixteen: %v", i, err)
		}
		if seen[out.Label] {
			t.Fatalf("label %d was handed out while still outstanding", out.Label)
		}
		seen[out.Label] = true
	}

	if _, err := c.SetVolume(1); err == nil {
		t.Error("a seventeenth command went out on sixteen labels")
	}
}

// A label comes back into use once its answer has arrived.
func TestALabelIsFreedByItsAnswer(t *testing.T) {
	c := New()

	var held []Transport
	for i := range 16 {
		out, err := c.SetVolume(i)
		if err != nil {
			t.Fatalf("command %d: %v", i, err)
		}
		held = append(held, out)
	}

	freed := held[3].Label
	if _, err := c.Receive(respond(held[3], Accepted, SetAbsoluteVolume(1))); err != nil {
		t.Fatalf("the answer: %v", err)
	}

	out, err := c.SetVolume(7)
	if err != nil {
		t.Fatalf("no label although one was freed: %v", err)
	}
	if out.Label != freed {
		t.Errorf("went out on label %d, want the freed %d", out.Label, freed)
	}
}

// An answer to a question nobody asked is not an error — a label may have been given up on — but
// there is nothing to do with it.
func TestAnAnswerToNothing(t *testing.T) {
	c := New()

	out, err := c.Receive(respond(Transport{Label: 9}, Stable, GetPlayStatus()))
	if err != nil {
		t.Errorf("an unmatched answer gave %v", err)
	}
	if len(out) != 0 {
		t.Errorf("%d commands", len(out))
	}
}

func TestAButtonSendsBothHalves(t *testing.T) {
	c := New()

	out, err := c.Press(OpPause)
	if err != nil {
		t.Fatalf("Press: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("%d messages, want a press and a release", len(out))
	}

	for i, want := range []bool{Pressed, Released} {
		frame, err := ParseAVC(out[i].Payload)
		if err != nil {
			t.Fatalf("ParseAVC: %v", err)
		}
		op, released, err := Button(frame)
		if err != nil {
			t.Fatalf("Button: %v", err)
		}
		if op != OpPause {
			t.Errorf("message %d is button %#02x", i, op)
		}
		if released != want {
			t.Errorf("message %d has released=%v, want %v", i, released, want)
		}
	}
}

// A progress bar drawn from notifications alone steps once a second rather than sweeping.
func TestElapsedCarriesThePositionForward(t *testing.T) {
	s := State{Progress: Progress{
		Length:   215 * time.Second,
		Position: 42 * time.Second,
		Status:   StatusPlaying,
	}}

	if got := s.Elapsed(400 * time.Millisecond); got != 42400*time.Millisecond {
		t.Errorf("elapsed %v", got)
	}

	// Never past the end, whatever the clock says.
	if got := s.Elapsed(time.Hour); got != 215*time.Second {
		t.Errorf("elapsed %v, want the length", got)
	}
}

// A paused track does not advance, however long ago the position was heard.
func TestElapsedStandsStillWhenPaused(t *testing.T) {
	s := State{Progress: Progress{Position: 42 * time.Second, Status: StatusPaused}}

	if got := s.Elapsed(time.Minute); got != 42*time.Second {
		t.Errorf("a paused track advanced to %v", got)
	}
}

// A phone that goes away leaves nothing behind, or the next one inherits its track.
func TestForgettingAPhone(t *testing.T) {
	c := New()
	out, _ := c.Start()
	c.Receive(respond(out[1], Stable, PDU{
		ID:     PDUGetElementAttributes,
		Params: elementAttributes(attribute(AttrTitle, "Sun Ra")),
	}))

	c.Forget()

	if got := c.State(); got.Track.Title != "" || got.Volume != -1 {
		t.Errorf("state survived: %+v", got)
	}

	// And the registrations are gone, so the next phone is asked again.
	again, _ := c.register(EventPlaybackStatus)
	if len(again) != 1 {
		t.Errorf("%d registrations after forgetting, want one", len(again))
	}
}

// registers is the phone asking to be told when this device's volume moves.
func registers(t *testing.T, c *Controller, label byte) {
	t.Helper()

	body := PDU{ID: PDURegisterNotification, Params: []byte{EventVolume, 0, 0, 0, 0}}
	out, err := c.Receive(Transport{
		Label:   label,
		Type:    MessageCommand,
		Payload: body.Frame(Status).Marshal(),
	})
	if err != nil {
		t.Fatalf("registering: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("%d answers to a registration, want an interim", len(out))
	}

	frame, err := ParseAVC(out[0].Payload)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Code != Interim {
		t.Fatalf("a registration was answered %#x, want an interim", frame.Code)
	}
}

// The other half of absolute volume: this device's own volume moving, which the phone asked to be
// told about so its slider follows. Without it the phone shows one level and the room hears another.
func TestAVolumeMovedHereIsSentToThePhone(t *testing.T) {
	c := New()
	registers(t, c, 9)

	out := c.Volumed(70)
	if len(out) != 1 {
		t.Fatalf("%d messages for a volume that moved, want one", len(out))
	}
	if out[0].Label != 9 {
		t.Errorf("answered on label %d, want the one registered on", out[0].Label)
	}

	frame, err := ParseAVC(out[0].Payload)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Code != Changed {
		t.Errorf("sent %#x, want a changed", frame.Code)
	}

	p, err := ParsePDU(frame.Operands)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Params) < 2 || p.Params[0] != EventVolume {
		t.Fatalf("the notification is % x, want a volume one", p.Params)
	}
	if got := Percent(p.Params[1]); got < 69 || got > 71 {
		t.Errorf("told the phone %d%%, want about 70", got)
	}
}

// A registration is spent once answered. Sending a second changed against it would be answering a
// question the phone has not asked again.
func TestTheRegistrationIsGoodForOneAnswer(t *testing.T) {
	c := New()
	registers(t, c, 3)

	if out := c.Volumed(40); len(out) != 1 {
		t.Fatalf("%d messages for the first move, want one", len(out))
	}
	if out := c.Volumed(55); len(out) != 0 {
		t.Errorf("%d messages for a second move on a spent registration, want none", len(out))
	}

	// Until it asks again.
	registers(t, c, 4)
	if out := c.Volumed(60); len(out) != 1 {
		t.Errorf("%d messages after registering again, want one", len(out))
	}
}

// Answering the phone's own command back at it is how two ends chase each other up and down the
// scale: the phone sets 50, this device tells it 50, which it treats as a change, and so on.
func TestTheVolumeThePhoneJustSetIsNotSentBack(t *testing.T) {
	c := New()
	registers(t, c, 2)

	_, err := c.Receive(Transport{
		Label:   6,
		Type:    MessageCommand,
		Payload: SetAbsoluteVolume(Volume(50)).Frame(Control).Marshal(),
	})
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}

	if out := c.Volumed(c.State().Volume); len(out) != 0 {
		t.Errorf("%d messages echoing the phone's own volume back at it, want none", len(out))
	}
}

// Nobody asked, so there is nothing owed.
func TestAVolumeMovedWithNoRegistrationSaysNothing(t *testing.T) {
	c := New()

	if out := c.Volumed(80); len(out) != 0 {
		t.Errorf("%d messages with no registration outstanding, want none", len(out))
	}
}

// A new stream is new content, and what the last one was showing does not describe it.
//
// The track survives a blank answer on purpose — a phone between tracks sends one, and dropping the
// title each time makes the card flicker. Across a stream that reasoning inverts: an app that
// publishes no metadata at all, as a video app does not, would otherwise leave the previous app's
// song on the card while something else plays.
func TestClearedForgetsTheTrackButNotTheLink(t *testing.T) {
	c := New()

	registers(t, c, 4)
	c.state.Track = Track{Title: "Aja", Artist: "Steely Dan"}
	c.state.Progress = Progress{Status: StatusPlaying, Position: time.Minute}

	var said State
	c.Changed = func(s State) { said = s }

	c.Cleared()

	if got := c.State().Track; got.Title != "" || got.Artist != "" {
		t.Errorf("the track survived being cleared: %+v", got)
	}
	if got := c.State().Progress; got.Status != 0 || got.Position != 0 {
		t.Errorf("the progress survived being cleared: %+v", got)
	}
	if said.Track.Title != "" {
		t.Errorf("the screen was told %q", said.Track.Title)
	}

	// The volume and the registration belong to the link, not to the track.
	if got := c.State().Volume; got == 0 {
		t.Error("clearing a track reset the volume")
	}
	if out := c.Volumed(60); len(out) != 1 {
		t.Error("clearing a track dropped the phone's volume registration")
	}
}
