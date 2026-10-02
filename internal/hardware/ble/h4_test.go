package ble

import (
	"errors"
	"testing"
)

// H4 framing: one byte saying what a packet is, then the packet.
func TestAnEventIsReadOffTheFrontOfTheBuffer(t *testing.T) {
	buf := []byte{typeEvent, 0x0e, 0x01, 0xaa}

	got, took, err := parsePacket(buf)
	if err != nil {
		t.Fatalf("parsePacket: %v", err)
	}
	if got.kind != typeEvent {
		t.Errorf("kind %#02x, want an event", got.kind)
	}
	if got.event.Code != 0x0e {
		t.Errorf("code %#02x, want 0x0e", got.event.Code)
	}
	if string(got.event.Params) != "\xaa" {
		t.Errorf("params % x", got.event.Params)
	}
	if took != len(buf) {
		t.Errorf("took %d of %d", took, len(buf))
	}
}

// ACL is the other thing on the line, and it is read as itself rather than as a short event.
func TestAnACLPacketIsReadAsData(t *testing.T) {
	// Handle 1, first fragment, four bytes of payload carrying a two byte L2CAP header.
	buf := []byte{typeACL, 0x01, 0x20, 0x04, 0x00, 0x00, 0x00, 0x01, 0x00}

	got, took, err := parsePacket(buf)
	if err != nil {
		t.Fatalf("parsePacket: %v", err)
	}
	if got.kind != typeACL {
		t.Fatalf("kind %#02x, want acl", got.kind)
	}
	if took != len(buf) {
		t.Errorf("took %d of %d", took, len(buf))
	}
}

// Anything that is neither is refused rather than read as a short one of either.
func TestAPacketThatIsNeither(t *testing.T) {
	if _, _, err := parsePacket([]byte{typeCommand, 0x03, 0x0c, 0x00}); err == nil {
		t.Error("a command was read back as something the chip had sent")
	}
}

func TestAPacketThatHasNotAllArrived(t *testing.T) {
	for _, buf := range [][]byte{
		{},
		{typeEvent},
		{typeEvent, 0x01},
		{typeEvent, 0x01, 0x04, 0x01, 0x02},
	} {
		if _, _, err := parsePacket(buf); !errors.Is(err, errShort) {
			t.Errorf("% x gave %v, want a short read", buf, err)
		}
	}
}

// Several packets back to back, which is what a busy line hands over in one read.
func TestPacketsBackToBack(t *testing.T) {
	buf := []byte{
		typeEvent, 0x0e, 0x01, 0xaa,
		typeEvent, 0x3e, 0x02, 0xbb, 0xcc,
		typeEvent, 0x0f, 0x01, 0xdd,
	}

	for i, want := range []byte{0x0e, 0x3e, 0x0f} {
		got, took, err := parsePacket(buf)
		if err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}
		if got.event.Code != want {
			t.Errorf("packet %d is %#02x, want %#02x", i, got.event.Code, want)
		}
		buf = buf[took:]
	}

	if len(buf) != 0 {
		t.Errorf("%d bytes left over", len(buf))
	}
}

// resync is the last resort: it finds the start of the next event and drops what was in front.
func TestResyncFindsTheNextEvent(t *testing.T) {
	got := resync([]byte{0xaa, 0xbb, typeEvent, 0x01, 0x00})

	if len(got) != 3 || got[0] != typeEvent {
		t.Errorf("resync left % x", got)
	}
}

// Nothing recognisable at all leaves nothing behind, rather than the same rubbish for ever.
func TestResyncOnNothingRecognisable(t *testing.T) {
	if got := resync([]byte{0xaa, 0xbb, 0xcc}); len(got) != 0 {
		t.Errorf("left % x", got)
	}
}

// An event that arrives while a reader is waiting for a different one is kept, since dropping it
// loses whichever answer happened to land first.
func TestAnEventNobodyAskedForIsKept(t *testing.T) {
	var q pending

	q.add(packet{kind: typeEvent, event: event{Code: 0x3e, Params: []byte{0xbb}}})

	got, ok := q.take()
	if !ok {
		t.Fatal("the event was not kept")
	}
	if got.Code != 0x3e {
		t.Errorf("code %#02x", got.Code)
	}
	if _, ok := q.take(); ok {
		t.Error("the same event came back twice")
	}
}

// In order: a command status and the event that followed it are not interchangeable.
func TestTheQueueKeepsItsOrder(t *testing.T) {
	var q pending

	for _, code := range []byte{0x01, 0x02, 0x03} {
		q.add(packet{kind: typeEvent, event: event{Code: code}})
	}
	for _, want := range []byte{0x01, 0x02, 0x03} {
		got, ok := q.take()
		if !ok || got.Code != want {
			t.Errorf("gave %#02x %v, want %#02x", got.Code, ok, want)
		}
	}
}

// An empty queue says so rather than handing back a zero event, which would read as a valid one
// with code zero.
func TestTakingFromAnEmptyQueue(t *testing.T) {
	var q pending

	if _, ok := q.take(); ok {
		t.Error("an empty queue produced an event")
	}
}
