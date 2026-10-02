package sdp

import "testing"

// A request this side writes has to be one this side can read: the two halves are the same shape
// seen from either end, and a disagreement between them is a disagreement with every phone too.
func TestARequestRoundTrips(t *testing.T) {
	want := Search{
		Pattern:    []uint32{UUIDAVRemoteControl},
		MaxBytes:   0x03f0,
		Attributes: []AttrRange{{First: AttrServiceClasses, Last: AttrServiceClasses}, {First: 0x0004, Last: 0x0009}},
	}

	p, err := want.Request(7)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if p.ID != PDUSearchAttributeRequest {
		t.Fatalf("pdu %#02x, want a search attribute request", p.ID)
	}
	if p.Transaction != 7 {
		t.Errorf("transaction %d, want 7", p.Transaction)
	}

	got, err := ParseSearchAttribute(p.Params)
	if err != nil {
		t.Fatalf("ParseSearchAttribute: %v", err)
	}

	if len(got.Pattern) != 1 || got.Pattern[0] != UUIDAVRemoteControl {
		t.Errorf("pattern %#x", got.Pattern)
	}
	if got.MaxBytes != 0x03f0 {
		t.Errorf("cap %d, want 1008", got.MaxBytes)
	}
	if len(got.Attributes) != 2 {
		t.Fatalf("%d attribute ranges, want two", len(got.Attributes))
	}
	if got.Attributes[1] != (AttrRange{First: 0x0004, Last: 0x0009}) {
		t.Errorf("the span came back as %+v", got.Attributes[1])
	}
}

// A continuation is echoed back so the far end knows where it left off. Dropping it asks for the
// beginning again, which is a fetch that never ends.
func TestAContinuationIsCarriedBack(t *testing.T) {
	s := Search{Pattern: []uint32{UUIDAudioSink}, Continuation: []byte{0xaa, 0xbb}}

	p, err := s.Request(1)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}

	got, err := ParseSearchAttribute(p.Params)
	if err != nil {
		t.Fatalf("ParseSearchAttribute: %v", err)
	}
	if string(got.Continuation) != "\xaa\xbb" {
		t.Errorf("continuation % x", got.Continuation)
	}
}

// An answer this side writes has to be one this side can read, which also says the records survive
// the trip with their attributes intact.
func TestAnAnswerRoundTripsIntoRecords(t *testing.T) {
	want := RemoteControl(2, "Lanovo", 0x0017, 0x001b, UUIDAVRemoteControlController, 0x03cf)

	p, err := SearchAttributeResponse(3, []Record{want})
	if err != nil {
		t.Fatalf("SearchAttributeResponse: %v", err)
	}

	records, more, err := ParseSearchAttributeResponse(p.Params)
	if err != nil {
		t.Fatalf("ParseSearchAttributeResponse: %v", err)
	}
	if len(more) != 0 {
		t.Errorf("a continuation of % x came back from an answer that fitted", more)
	}
	if len(records) != 1 {
		t.Fatalf("%d records, want one", len(records))
	}

	got := records[0]
	if len(got) != len(want) {
		t.Errorf("%d attributes back, want %d", len(got), len(want))
	}

	features, ok := got.Attribute(AttrFeatures)
	if !ok {
		t.Fatal("the supported features did not survive")
	}
	if v, _ := features.Uint(); v != 0x03cf {
		t.Errorf("features %#04x, want 0x03cf", v)
	}
}

// The whole point of asking: the channel a protocol is served on lives in the additional protocol
// descriptor lists, not the main one.
func TestFindingAChannelInTheExtraLists(t *testing.T) {
	r := RemoteControl(2, "Lanovo", 0x0017, 0x001b, UUIDAVRemoteControlController, 0x03cf)

	psm, ok := MorePSM(r, UUIDAVCTP)
	if !ok {
		t.Fatal("the browsing channel was not found")
	}
	if psm != 0x001b {
		t.Errorf("browsing on %#04x, want 0x001b", psm)
	}

	// Cover art would be an OBEX list, which this record does not have. Naming a protocol that is
	// not there must not return the first channel it finds.
	if got, ok := MorePSM(r, UUIDOBEX); ok {
		t.Errorf("a protocol that is not published came back on %#04x", got)
	}
}

// A record with no extra lists at all is the common case and not an error.
func TestARecordWithNoExtraLists(t *testing.T) {
	r := AudioSink(1, "Lanovo", 0x0019, FeatureSpeaker)

	if _, ok := MorePSM(r, UUIDOBEX); ok {
		t.Error("a channel came back from a record that publishes none")
	}
}

// A truncated answer is refused rather than read past.
func TestAnAnswerShorterThanItClaims(t *testing.T) {
	if _, _, err := ParseSearchAttributeResponse([]byte{0x00, 0x40, 0x35, 0x00}); err == nil {
		t.Error("an answer claiming 64 bytes of records with two present was accepted")
	}
}

func TestAnAnswerTooShortToHaveAHeader(t *testing.T) {
	if _, _, err := ParseSearchAttributeResponse([]byte{0x00}); err == nil {
		t.Error("one byte parsed as an answer")
	}
}

// An odd number of fields cannot be identifier and value pairs, so it is a truncated record rather
// than one with something missing.
func TestARecordThatCannotPairUp(t *testing.T) {
	body, err := Sequence(Sequence(Uint16(AttrName), Text("Lanovo"), Uint16(AttrFeatures))).Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	params := append([]byte{byte(len(body) >> 8), byte(len(body))}, body...)
	params = append(params, 0)

	if _, _, err := ParseSearchAttributeResponse(params); err == nil {
		t.Error("a record with three fields was accepted")
	}
}
