package cast

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// same compares two messages, which the type itself cannot do because it carries a byte slice.
func same(a, b Message) bool {
	return a.Source == b.Source &&
		a.Destination == b.Destination &&
		a.Namespace == b.Namespace &&
		a.Payload == b.Payload &&
		bytes.Equal(a.Binary, b.Binary)
}

// The encoding is written by hand rather than generated, so the first thing it owes is a check
// against bytes somebody else would produce. This is a CONNECT from sender-0 to receiver-0, field
// by field, worked out from cast_channel.proto rather than from this package.
func TestAMessageEncodesToTheBytesTheSchemaSays(t *testing.T) {
	m := Message{
		Source:      "sender-0",
		Destination: "receiver-0",
		Namespace:   NSConnection,
		Payload:     `{"type":"CONNECT"}`,
	}

	var want []byte
	want = append(want, 0x08, 0x00) // 1: protocol version, varint 0
	want = append(want, 0x12, 8)    // 2: source, 8 bytes
	want = append(want, "sender-0"...)
	want = append(want, 0x1a, 10) // 3: destination, 10 bytes
	want = append(want, "receiver-0"...)
	want = append(want, 0x22, byte(len(NSConnection))) // 4: namespace
	want = append(want, NSConnection...)
	want = append(want, 0x28, 0x00)                 // 5: payload type, varint 0 for string
	want = append(want, 0x32, byte(len(m.Payload))) // 6: the json
	want = append(want, m.Payload...)

	if got := m.Marshal(); !bytes.Equal(got, want) {
		t.Errorf("came back\n% x\nwant\n% x", got, want)
	}
}

func TestAMessageRoundTrips(t *testing.T) {
	want := Message{
		Source:      "receiver-0",
		Destination: "sender-0",
		Namespace:   NSReceiver,
		Payload:     `{"type":"RECEIVER_STATUS"}`,
	}

	got, err := Unmarshal(want.Marshal())
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !same(got, want) {
		t.Errorf("came back %+v, want %+v", got, want)
	}
}

// The payload type has to say binary when the binary field is set, or a sender reads the empty
// string field as the payload and finds nothing in it.
func TestABinaryPayloadSaysItIsBinary(t *testing.T) {
	m := Message{Namespace: NSDeviceAuth, Binary: []byte{0x01, 0x02}}

	raw := m.Marshal()
	if !bytes.Contains(raw, []byte{0x28, PayloadBinary}) {
		t.Errorf("the payload type is not binary: % x", raw)
	}

	got, err := Unmarshal(raw)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !bytes.Equal(got.Binary, m.Binary) {
		t.Errorf("binary came back % x", got.Binary)
	}
}

// The string field goes out even when empty. It is required in the schema, and a sender built
// against a strict parser refuses a message that leaves it out.
func TestTheStringPayloadIsAlwaysWritten(t *testing.T) {
	raw := Message{Source: "a", Destination: "b", Namespace: "c"}.Marshal()

	if !bytes.Contains(raw, []byte{0x32, 0x00}) {
		t.Errorf("the empty payload field was left out: % x", raw)
	}
}

// The schema has grown before, and a receiver that refuses a message for carrying something it does
// not know about is one that stops working when the sender is updated.
func TestAnUnknownFieldIsSkipped(t *testing.T) {
	raw := Message{Source: "a", Namespace: "n", Payload: "{}"}.Marshal()

	// Field 15, varint, which this does not know about.
	raw = append(raw, 0x78, 0x2a)

	got, err := Unmarshal(raw)
	if err != nil {
		t.Fatalf("an unknown field was refused: %v", err)
	}
	if got.Source != "a" || got.Namespace != "n" {
		t.Errorf("the known fields came back %+v", got)
	}
}

func TestATruncatedMessageIsRefused(t *testing.T) {
	raw := Message{Source: "sender-0", Namespace: NSHeartbeat, Payload: `{"type":"PING"}`}.Marshal()

	if _, err := Unmarshal(raw[:len(raw)-4]); err == nil {
		t.Error("a truncated message parsed")
	}
}

// Big-endian, where everything else in this tree that counts bytes is little. Getting it the usual
// way round gives a length of two billion and a reader that waits for the rest for ever.
func TestTheLengthPrefixIsBigEndian(t *testing.T) {
	framed := Frame(Message{Source: "a", Destination: "b", Namespace: "c", Payload: "{}"})

	body := framed[4:]
	if got := int(framed[0])<<24 | int(framed[1])<<16 | int(framed[2])<<8 | int(framed[3]); got != len(body) {
		t.Errorf("the prefix says %d and the body is %d", got, len(body))
	}
	if framed[0] != 0 || framed[1] != 0 {
		t.Errorf("the prefix looks little endian: % x", framed[:4])
	}
}

func TestAFramedMessageRoundTrips(t *testing.T) {
	want := Message{Source: "sender-0", Destination: "receiver-0", Namespace: NSHeartbeat, Payload: Ping()}

	got, n, err := Next(Frame(want))
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if n != len(Frame(want)) {
		t.Errorf("took %d bytes", n)
	}
	if !same(got, want) {
		t.Errorf("came back %+v", got)
	}
}

// Two in one read, which is what a sender that pipelines gives.
func TestTwoMessagesInOneBuffer(t *testing.T) {
	buf := append(
		Frame(Message{Namespace: NSHeartbeat, Payload: Ping()}),
		Frame(Message{Namespace: NSReceiver, Payload: `{"type":"GET_STATUS"}`})...,
	)

	first, n, err := Next(buf)
	if err != nil {
		t.Fatalf("the first: %v", err)
	}
	if first.Namespace != NSHeartbeat {
		t.Errorf("the first is %q", first.Namespace)
	}

	second, _, err := Next(buf[n:])
	if err != nil {
		t.Fatalf("the second: %v", err)
	}
	if second.Namespace != NSReceiver {
		t.Errorf("the second is %q", second.Namespace)
	}
}

// Half a message is the ordinary state of a stream being read, not a fault.
func TestAMessageThatHasNotAllArrived(t *testing.T) {
	framed := Frame(Message{Namespace: NSReceiver, Payload: `{"type":"GET_STATUS"}`})

	for _, n := range []int{0, 1, 3, 4, len(framed) - 1} {
		if _, _, err := Next(framed[:n]); !errors.Is(err, ErrShort) {
			t.Errorf("%d bytes gave %v, want a short read", n, err)
		}
	}
}

// The length arrives before the bytes do, so a wrong one otherwise asks this process to allocate
// whatever it says.
func TestARidiculousLengthIsRefused(t *testing.T) {
	buf := []byte{0xff, 0xff, 0xff, 0xff}

	if _, _, err := Next(buf); err == nil || errors.Is(err, ErrShort) {
		t.Errorf("a four gigabyte message gave %v, want a refusal", err)
	}
}

func TestReadingTheTypeOutOfAPayload(t *testing.T) {
	h, err := Kind(`{"type":"LAUNCH","requestId":7,"appId":"CC1AD845"}`)
	if err != nil {
		t.Fatalf("Kind: %v", err)
	}
	if h.Type != TypeLaunch || h.RequestID != 7 {
		t.Errorf("came back %+v", h)
	}
}

// A payload with no type is not something to guess about.
func TestAPayloadWithNoType(t *testing.T) {
	for _, p := range []string{`{}`, `{"requestId":3}`, `not json`, ``} {
		if _, err := Kind(p); err == nil {
			t.Errorf("%q was accepted", p)
		}
	}
}

// A request answered with the wrong id, or none, is a sender that waits out its timeout.
func TestAnAnswerCarriesTheRequestItAnswers(t *testing.T) {
	h, err := Kind(Status(42, ReceiverStatus{}))
	if err != nil {
		t.Fatalf("Kind: %v", err)
	}
	if h.Type != TypeStatus {
		t.Errorf("type %q", h.Type)
	}
	if h.RequestID != 42 {
		t.Errorf("request %d, want 42", h.RequestID)
	}
}

func TestAnUnsolicitedStatusCarriesAZeroRequestID(t *testing.T) {
	if got := Status(0, ReceiverStatus{}); !strings.Contains(got, `"requestId":0`) {
		t.Errorf("came back %q", got)
	}
}

// The status goes under a key of its own rather than beside the header, which is what a sender
// reads it out of.
func TestAStatusIsNestedUnderItsOwnKey(t *testing.T) {
	level := 0.5
	raw := Status(1, ReceiverStatus{
		Volume: Volume{Level: &level, ControlType: "attenuation"},
		Applications: []Application{{
			AppID:       "CC1AD845",
			SessionID:   "s1",
			TransportID: "s1",
			DisplayName: "Default Media Receiver",
			Namespaces:  []Namespace{{Name: NSMedia}},
		}},
	})

	var got struct {
		Type   string         `json:"type"`
		Status ReceiverStatus `json:"status"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("the status did not parse: %v", err)
	}

	if got.Status.Volume.Level == nil || *got.Status.Volume.Level != 0.5 {
		t.Errorf("the volume came back %+v", got.Status.Volume)
	}
	if len(got.Status.Applications) != 1 {
		t.Fatalf("%d applications", len(got.Status.Applications))
	}
	if got.Status.Applications[0].Namespaces[0].Name != NSMedia {
		t.Error("the application did not say it speaks the media namespace")
	}
}

// Nothing running is an empty list, which a sender reads as the device being free. A null there is
// read by some senders as an error instead.
func TestNothingRunningLeavesTheApplicationsOut(t *testing.T) {
	if got := Status(1, ReceiverStatus{}); strings.Contains(got, "applications") {
		t.Errorf("came back %q", got)
	}
}

// The cheapest message here and the one that must never be missed: a sender that does not get a
// pong inside its own timeout drops the connection.
func TestAPongIsAPong(t *testing.T) {
	h, err := Kind(Pong())
	if err != nil {
		t.Fatalf("Kind: %v", err)
	}
	if h.Type != TypePong {
		t.Errorf("came back %q", h.Type)
	}
}

func TestParsingALaunch(t *testing.T) {
	got, err := ParseLaunch(`{"type":"LAUNCH","requestId":3,"appId":"CC1AD845"}`)
	if err != nil {
		t.Fatalf("ParseLaunch: %v", err)
	}
	if got.AppID != "CC1AD845" || got.RequestID != 3 {
		t.Errorf("came back %+v", got)
	}
}

func TestALaunchNamingNothing(t *testing.T) {
	if _, err := ParseLaunch(`{"type":"LAUNCH","requestId":3}`); err == nil {
		t.Error("a launch with no application was accepted")
	}
}

// Level and Muted are pointers so a request carrying only one does not read as setting the other to
// zero — the difference between muting a speaker and turning it all the way down.
func TestAMuteDoesNotReadAsSilence(t *testing.T) {
	got, err := ParseSetVolume(`{"type":"SET_VOLUME","requestId":4,"volume":{"muted":true}}`)
	if err != nil {
		t.Fatalf("ParseSetVolume: %v", err)
	}
	if got.Volume.Level != nil {
		t.Errorf("a mute carried a level of %v", *got.Volume.Level)
	}
	if got.Volume.Muted == nil || !*got.Volume.Muted {
		t.Error("the mute did not come through")
	}
}

func TestSettingTheLevelAlone(t *testing.T) {
	got, err := ParseSetVolume(`{"type":"SET_VOLUME","requestId":4,"volume":{"level":0.25}}`)
	if err != nil {
		t.Fatalf("ParseSetVolume: %v", err)
	}
	if got.Volume.Muted != nil {
		t.Error("a level carried a mute")
	}
	if got.Volume.Level == nil || *got.Volume.Level != 0.25 {
		t.Errorf("the level came back %v", got.Volume.Level)
	}
}

func TestAVolumeThatSetsNothing(t *testing.T) {
	if _, err := ParseSetVolume(`{"type":"SET_VOLUME","requestId":4,"volume":{}}`); err == nil {
		t.Error("a volume setting neither level nor mute was accepted")
	}
}

// Out of range is refused rather than clamped: a sender asking for 11 has misunderstood the scale,
// and doing something sensible with it hides that.
func TestAVolumeOutsideItsRange(t *testing.T) {
	for _, p := range []string{
		`{"type":"SET_VOLUME","volume":{"level":1.5}}`,
		`{"type":"SET_VOLUME","volume":{"level":-0.1}}`,
	} {
		if _, err := ParseSetVolume(p); err == nil {
			t.Errorf("%q was accepted", p)
		}
	}
}

// A sender waiting on a reply holds the connection and eventually drops the whole thing, which
// reads in the room as the device refusing to be cast to at all.
func TestARefusalCarriesItsReasonAndItsRequest(t *testing.T) {
	var got struct {
		Header
		Reason string `json:"reason"`
	}

	if err := json.Unmarshal([]byte(Invalid(9, ReasonNotSupported)), &got); err != nil {
		t.Fatalf("the refusal did not parse: %v", err)
	}
	if got.Type != TypeInvalid || got.RequestID != 9 || got.Reason != ReasonNotSupported {
		t.Errorf("came back %+v", got)
	}

	if err := json.Unmarshal([]byte(LaunchError(10, ReasonUnknownAppID)), &got); err != nil {
		t.Fatalf("the launch error did not parse: %v", err)
	}
	if got.Type != TypeLaunchFail || got.RequestID != 10 {
		t.Errorf("came back %+v", got)
	}
}

// Everything this package produces has to be a payload something can read the type out of.
func TestEveryPayloadThisBuildsHasAType(t *testing.T) {
	for _, p := range []string{
		Connect(),
		Close(),
		Ping(),
		Pong(),
		Status(1, ReceiverStatus{}),
		Invalid(1, ReasonInvalidParams),
		LaunchError(1, ReasonUnknownAppID),
	} {
		if _, err := Kind(p); err != nil {
			t.Errorf("%q: %v", p, err)
		}
	}
}
