package cast

import (
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func payloadFields(t *testing.T, b []byte) (utf8, binary bool) {
	t.Helper()
	for len(b) > 0 {
		number, kind, n := protowire.ConsumeTag(b)
		if n < 0 {
			t.Fatal(protowire.ParseError(n))
		}
		b = b[n:]
		switch number {
		case fieldPayloadUTF8:
			utf8 = true
		case fieldPayloadBinary:
			binary = true
		}
		b = b[protowire.ConsumeFieldValue(number, kind, b):]
	}
	return utf8, binary
}

func TestABinaryMessageCarriesNoStringPayload(t *testing.T) {
	utf8, binary := payloadFields(t, Message{Source: "receiver-0", Destination: "sender-0", Namespace: NSDeviceAuth, Binary: []byte{0x12, 0x00}}.Marshal())
	if utf8 || !binary {
		t.Errorf("binary message: utf8 field %v, binary field %v", utf8, binary)
	}

	utf8, binary = payloadFields(t, Message{Source: "receiver-0", Destination: "sender-0", Namespace: NSReceiver}.Marshal())
	if !utf8 || binary {
		t.Errorf("empty string message: utf8 field %v, binary field %v", utf8, binary)
	}
}

func TestABinaryMessageRoundTrips(t *testing.T) {
	in := Message{Source: "receiver-0", Destination: "sender-0", Namespace: NSDeviceAuth, Binary: []byte{0x12, 0x00}}
	out, err := Unmarshal(in.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	if string(out.Binary) != string(in.Binary) || out.Payload != "" || out.Namespace != in.Namespace {
		t.Errorf("read back %+v", out)
	}
}
