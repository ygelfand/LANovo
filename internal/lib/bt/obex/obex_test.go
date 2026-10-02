package obex

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// A connect carries four fixed bytes before its headers, which nothing else does.
func TestAConnectCarriesItsVersionAndSize(t *testing.T) {
	raw := Connect(672, CoverArtTarget).Marshal()

	if raw[0] != OpConnect {
		t.Fatalf("opcode %#02x, want %#02x", raw[0], OpConnect)
	}
	if got := binary.BigEndian.Uint16(raw[1:]); int(got) != len(raw) {
		t.Errorf("the length says %d and the packet is %d", got, len(raw))
	}
	if raw[3] != Version {
		t.Errorf("version %#02x, want %#02x", raw[3], Version)
	}
	if got := binary.BigEndian.Uint16(raw[5:]); got != 672 {
		t.Errorf("mtu %d, want 672", got)
	}

	headers, err := ParseHeaders(raw[7:])
	if err != nil {
		t.Fatalf("ParseHeaders: %v", err)
	}

	target, ok := Find(headers, HeaderTarget)
	if !ok {
		t.Fatal("no target, so a far end with several services cannot tell which is wanted")
	}
	if !bytes.Equal(target.Value, CoverArtTarget) {
		t.Errorf("target % x", target.Value)
	}
}

// The answer to a connect hands back the identifier every later request carries.
func TestReadingTheAnswerToAConnect(t *testing.T) {
	raw := []byte{OK, 0, 0, Version, 0, 0x02, 0x00}
	raw = append(raw, Uint32(HeaderConnection, 0x12345678).Marshal()...)
	binary.BigEndian.PutUint16(raw[1:], uint16(len(raw)))

	p, err := ParseAnswer(raw, true)
	if err != nil {
		t.Fatalf("ParseAnswer: %v", err)
	}
	if p.Code != OK {
		t.Errorf("code %#02x, want %#02x", p.Code, OK)
	}
	if p.MTU != 512 {
		t.Errorf("mtu %d, want 512", p.MTU)
	}

	id, ok := p.Connection()
	if !ok {
		t.Fatal("no connection identifier came back")
	}
	if id != 0x12345678 {
		t.Errorf("connection %#08x", id)
	}
}

// A handle is seven ascii digits and travels as utf-16, null terminated. Sixteen bytes, and a
// target that is handed eight will not find the image.
func TestAHandleIsSixteenBytesOfUnicode(t *testing.T) {
	h := Text(HeaderName, "2964560")

	if len(h.Value) != 16 {
		t.Fatalf("%d bytes for a seven character handle, want 16", len(h.Value))
	}
	if h.Value[0] != 0x00 || h.Value[1] != '2' {
		t.Errorf("the first character came out as % x, want 00 32", h.Value[:2])
	}
	if h.Value[14] != 0 || h.Value[15] != 0 {
		t.Errorf("it does not end with a null pair: % x", h.Value[12:])
	}
}

// The whole of asking for a thumbnail, which is the request the player card makes.
func TestAskingForAThumbnail(t *testing.T) {
	raw := Thumbnail(0x0a0b0c0d, "2964560").Marshal()

	if raw[0] != OpGet|Final {
		t.Fatalf("opcode %#02x, want a final get", raw[0])
	}

	headers, err := ParseHeaders(raw[packetHeader:])
	if err != nil {
		t.Fatalf("ParseHeaders: %v", err)
	}

	conn, ok := Find(headers, HeaderConnection)
	if !ok {
		t.Fatal("no connection identifier, so the target does not know which session")
	}
	if v, _ := conn.Uint32(); v != 0x0a0b0c0d {
		t.Errorf("connection %#08x", v)
	}

	kind, ok := Find(headers, HeaderType)
	if !ok {
		t.Fatal("no type, so the target does not know what is being asked for")
	}
	if string(kind.Value) != TypeThumbnail+"\x00" {
		t.Errorf("type %q, want it null terminated", kind.Value)
	}

	// The image handle header is what a target reads the handle from.
	if _, ok := Find(headers, HeaderImgHandle); !ok {
		t.Error("the handle is not in the image handle header")
	}
	if _, ok := Find(headers, HeaderName); ok {
		t.Error("a name header went out, which the handle does not belong in")
	}
}

// A long image arrives over several answers: body, body, then end of body.
func TestABodyArrivingInPieces(t *testing.T) {
	first, err := ParseAnswer(answer(Continue, Bytes(HeaderBody, []byte("abc"))), false)
	if err != nil {
		t.Fatalf("ParseAnswer: %v", err)
	}

	data, done := first.Body()
	if string(data) != "abc" || done {
		t.Fatalf("first piece %q done=%v", data, done)
	}

	last, err := ParseAnswer(answer(OK, Bytes(HeaderEndOfBody, []byte("de"))), false)
	if err != nil {
		t.Fatalf("ParseAnswer: %v", err)
	}

	data, done = last.Body()
	if string(data) != "de" || !done {
		t.Fatalf("last piece %q done=%v", data, done)
	}
}

// The request for the rest carries the session and nothing else. Repeating the name would start
// the fetch over instead of continuing it.
func TestAskingForTheRestCarriesOnlyTheSession(t *testing.T) {
	headers, err := ParseHeaders(More(7).Marshal()[packetHeader:])
	if err != nil {
		t.Fatalf("ParseHeaders: %v", err)
	}

	if len(headers) != 1 {
		t.Fatalf("%d headers, want only the session", len(headers))
	}
	if headers[0].ID != HeaderConnection {
		t.Errorf("header %#02x, want the connection", headers[0].ID)
	}
}

// Each encoding is a different width, and a reader that guesses wrong reads the next header's
// identifier as data.
func TestHeadersOfEveryEncodingRoundTrip(t *testing.T) {
	want := []Header{
		Text(HeaderName, "hi"),
		Bytes(HeaderType, []byte("x-bt/img-thm\x00")),
		{ID: HeaderDescription | 0x80, Value: []byte{0x05}},
		Uint32(HeaderConnection, 0xdeadbeef),
	}

	var raw []byte
	for _, h := range want {
		raw = append(raw, h.Marshal()...)
	}

	got, err := ParseHeaders(raw)
	if err != nil {
		t.Fatalf("ParseHeaders: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("%d headers back, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i].ID != want[i].ID || !bytes.Equal(got[i].Value, want[i].Value) {
			t.Errorf("header %d came back as %#02x % x, want %#02x % x",
				i, got[i].ID, got[i].Value, want[i].ID, want[i].Value)
		}
	}
}

// A header claiming more than arrived is refused rather than read past.
func TestAHeaderLongerThanThePacket(t *testing.T) {
	if _, err := ParseHeaders([]byte{HeaderType, 0x00, 0x40, 0x01}); err == nil {
		t.Error("a header claiming 64 bytes with one present was accepted")
	}
}

// A length under the header size would have a reader looping on the same bytes.
func TestAHeaderClaimingLessThanItsOwnHeader(t *testing.T) {
	if _, err := ParseHeaders([]byte{HeaderType, 0x00, 0x01, 0xff}); err == nil {
		t.Error("a header claiming one byte was accepted")
	}
}

func TestAPacketShorterThanItsHeader(t *testing.T) {
	if _, err := ParseAnswer([]byte{OK, 0x00}, false); err == nil {
		t.Error("two bytes parsed as a packet")
	}
}

// answer builds a response packet with these headers.
func answer(code byte, headers ...Header) []byte {
	out := []byte{code, 0, 0}
	for _, h := range headers {
		out = append(out, h.Marshal()...)
	}
	binary.BigEndian.PutUint16(out[1:], uint16(len(out)))
	return out
}
