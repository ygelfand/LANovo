package sdp

import (
	"encoding/binary"
	"errors"
	"testing"
)

// searchRequest builds what a phone sends when it is looking for an audio sink.
func searchRequest(t *testing.T, pattern []Element, attrs []Element, max uint16) []byte {
	t.Helper()

	uuids, err := Sequence(pattern...).Marshal()
	if err != nil {
		t.Fatalf("Marshal pattern: %v", err)
	}
	list, err := Sequence(attrs...).Marshal()
	if err != nil {
		t.Fatalf("Marshal attributes: %v", err)
	}

	out := append([]byte{}, uuids...)
	out = binary.BigEndian.AppendUint16(out, max)
	out = append(out, list...)
	return append(out, 0) // an empty continuation
}

func TestAPDUGoesOutAndComesBack(t *testing.T) {
	want := PDU{ID: PDUSearchAttributeRequest, Transaction: 0x1234, Params: []byte{1, 2, 3}}

	got, err := ParsePDU(want.Marshal())
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}
	if got.ID != want.ID || got.Transaction != want.Transaction {
		t.Errorf("came back %+v, want %+v", got, want)
	}
	if string(got.Params) != string(want.Params) {
		t.Errorf("params came back %x", got.Params)
	}
}

func TestAPDUThatHasNotAllArrived(t *testing.T) {
	whole := PDU{ID: PDUError, Transaction: 1, Params: []byte{0, 3}}.Marshal()

	for n := range len(whole) {
		if _, err := ParsePDU(whole[:n]); !errors.Is(err, ErrShort) {
			t.Errorf("%d of %d bytes gave %v, want a short read", n, len(whole), err)
		}
	}
}

// The request a phone actually sends: find an audio sink, give me everything about it.
func TestReadingTheRequestAPhoneSends(t *testing.T) {
	params := searchRequest(t,
		[]Element{UUID16(UUIDAudioSink)},
		[]Element{AttrRange{First: 0x0000, Last: 0xffff}.Element()}, // the range 0x0000 to 0xffff, packed as one four byte value
		672,
	)

	got, err := ParseSearchAttribute(params)
	if err != nil {
		t.Fatalf("ParseSearchAttribute: %v", err)
	}

	if len(got.Pattern) != 1 || got.Pattern[0] != UUIDAudioSink {
		t.Errorf("the search pattern came back %v", got.Pattern)
	}
	if got.MaxBytes != 672 {
		t.Errorf("the cap came back %d, want 672", got.MaxBytes)
	}
	if len(got.Attributes) != 1 {
		t.Fatalf("%d attribute ranges came back, want one", len(got.Attributes))
	}
	if got.Attributes[0] != (AttrRange{First: 0x0000, Last: 0xffff}) {
		t.Errorf("the range came back %+v", got.Attributes[0])
	}
	if len(got.Continuation) != 0 {
		t.Errorf("a first request carried a continuation: %x", got.Continuation)
	}
}

// Two bytes is one attribute, four is a range. Reading a range as a single identifier means
// answering with one attribute where the phone asked for all of them.
func TestASingleAttributeAndARange(t *testing.T) {
	params := searchRequest(t,
		[]Element{UUID16(UUIDAudioSink)},
		[]Element{AttrRange{First: AttrName, Last: AttrName}.Element(), AttrRange{First: 0x0004, Last: 0x0009}.Element()},
		672,
	)

	got, err := ParseSearchAttribute(params)
	if err != nil {
		t.Fatalf("ParseSearchAttribute: %v", err)
	}
	if len(got.Attributes) != 2 {
		t.Fatalf("%d ranges came back, want two", len(got.Attributes))
	}
	if got.Attributes[0] != (AttrRange{First: AttrName, Last: AttrName}) {
		t.Errorf("a single identifier came back as %+v", got.Attributes[0])
	}
	if got.Attributes[1] != (AttrRange{First: 0x0004, Last: 0x0009}) {
		t.Errorf("a range came back as %+v", got.Attributes[1])
	}
}

// All of the pattern has to match, not any of it. Answering a search for two classes with something
// that is only one of them is how a device appears and then cannot be used.
func TestASearchWantsEveryClassItNamed(t *testing.T) {
	both := Search{Pattern: []uint32{UUIDAudioSink, UUIDA2DP}}

	if !both.Wants([]uint32{UUIDAudioSink, UUIDA2DP}) {
		t.Error("a record with both classes did not match")
	}
	if both.Wants([]uint32{UUIDAudioSink}) {
		t.Error("a record with only one of two classes matched")
	}

	one := Search{Pattern: []uint32{UUIDAudioSink}}
	if !one.Wants([]uint32{UUIDAudioSink, UUIDA2DP}) {
		t.Error("a record with more classes than asked for did not match")
	}
}

func TestOnlyTheAttributesAskedForComeBack(t *testing.T) {
	r := AudioSink(1, "LANovo", UUIDAVDTP, FeatureSpeaker)

	s := Search{Attributes: []AttrRange{{First: AttrProtocols, Last: AttrProtocols}}}
	got := s.Selected(r)

	if len(got) != 1 {
		t.Fatalf("%d attributes came back, want one", len(got))
	}
	if got[0].ID != AttrProtocols {
		t.Errorf("attribute %#x came back", got[0].ID)
	}

	whole := Search{Attributes: []AttrRange{{First: 0, Last: 0xffff}}}
	if len(whole.Selected(r)) != len(r) {
		t.Errorf("asking for everything gave %d of %d attributes", len(whole.Selected(r)), len(r))
	}
}

// The answer wraps every matching record's attribute list in one more sequence. Leaving that outer
// sequence off is what produces a phone that sees the device and finds no services on it.
func TestTheAnswerWrapsEveryRecordInOneSequence(t *testing.T) {
	r := AudioSink(1, "LANovo", UUIDAVDTP, FeatureSpeaker)

	pdu, err := SearchAttributeResponse(0x1234, []Record{r})
	if err != nil {
		t.Fatalf("SearchAttributeResponse: %v", err)
	}
	if pdu.ID != PDUSearchAttributeResponse {
		t.Errorf("answered with pdu %#x", pdu.ID)
	}
	if pdu.Transaction != 0x1234 {
		t.Errorf("the answer carried transaction %#x", pdu.Transaction)
	}

	// Two byte length, the body, then a continuation byte.
	if len(pdu.Params) < 3 {
		t.Fatalf("the answer carried %d bytes", len(pdu.Params))
	}
	n := int(binary.BigEndian.Uint16(pdu.Params))
	if got := len(pdu.Params) - 3; got != n {
		t.Errorf("the length says %d and the body is %d", n, got)
	}
	if pdu.Params[len(pdu.Params)-1] != 0 {
		t.Error("the continuation is not empty")
	}

	outer, _, err := ParseElement(pdu.Params[2 : 2+n])
	if err != nil {
		t.Fatalf("ParseElement: %v", err)
	}
	if outer.Type != TypeSequence {
		t.Fatalf("the body is type %d, want a sequence of records", outer.Type)
	}
	if len(outer.Children) != 1 {
		t.Fatalf("%d records came back, want one", len(outer.Children))
	}

	// And the record inside it is still readable as one.
	inner := outer.Children[0]
	if len(inner.Children) != len(r)*2 {
		t.Errorf("the record has %d children, want %d", len(inner.Children), len(r)*2)
	}
}

func TestAnErrorAnswer(t *testing.T) {
	pdu := Error(0x0042, ErrorBadSyntax)

	if pdu.ID != PDUError || pdu.Transaction != 0x0042 {
		t.Errorf("came out %+v", pdu)
	}
	if got := binary.BigEndian.Uint16(pdu.Params); got != ErrorBadSyntax {
		t.Errorf("the reason came out %#x", got)
	}
}

func TestRequestsThatDoNotMakeSense(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params []byte
	}{
		{"nothing", nil},
		{"a pattern that is not a sequence", func() []byte {
			b, _ := Uint(1).Marshal()
			return b
		}()},
		{"no cap after the pattern", func() []byte {
			b, _ := Sequence(UUID16(UUIDAudioSink)).Marshal()
			return b
		}()},
		{"no continuation", func() []byte {
			p, _ := Sequence(UUID16(UUIDAudioSink)).Marshal()
			l, _ := Sequence(AttrRange{First: 0, Last: 0xffff}.Element()).Marshal()
			out := append(p, 0x02, 0xa0)
			return append(out, l...)
		}()},
	} {
		if _, err := ParseSearchAttribute(tc.params); err == nil {
			t.Errorf("%s: was accepted", tc.name)
		}
	}
}
