package sdp

import "testing"

// Data elements nest, and the parser recurses to follow them. A phone is not the only thing that
// can open the discovery channel, so the depth has to come from the bytes rather than from trust.

func FuzzParseElement(f *testing.F) {
	raw, _ := Sequence(UUID16(UUIDAudioSink), Uint(0x0019), Text("Lanovo")).Marshal()
	f.Add(raw)

	// A sequence that says it holds far more than it does.
	f.Add([]byte{0x35, 0xff, 0x01})
	f.Add([]byte{})

	// Nesting, which is where a recursive parser goes over the side.
	deep := make([]byte, 0, 4096)
	for range 2048 {
		deep = append(deep, 0x35, 0x02)
	}
	f.Add(deep)

	f.Fuzz(func(t *testing.T, raw []byte) {
		e, n, err := ParseElement(raw)
		if err != nil {
			return
		}
		if n <= 0 || n > len(raw) {
			t.Fatalf("an element of %d bytes was taken out of %d", n, len(raw))
		}

		// Anything that parsed has to be writable again, since a record read off the wire is
		// answered with parts of itself.
		e.Marshal()
	})
}

func FuzzParseSearchAttribute(f *testing.F) {
	pattern, _ := Sequence(UUID16(UUIDAudioSink)).Marshal()
	list, _ := Sequence(AttrRange{First: 0, Last: 0xffff}.Element()).Marshal()

	params := append([]byte(nil), pattern...)
	params = append(params, 0xff, 0xff)
	params = append(params, list...)
	params = append(params, 0)
	f.Add(params)

	f.Add([]byte{0x35, 0x03, 0x19, 0x11, 0x0b})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, raw []byte) {
		s, err := ParseSearchAttribute(raw)
		if err != nil {
			return
		}

		// Whatever came back is used to select attributes out of a record, so it has to be safe to
		// ask it questions.
		s.Wants([]uint32{UUIDAudioSink})
		s.Selected(AudioSink(1, "Lanovo", 0x0019, FeatureSpeaker))
	})
}

func FuzzParsePDU(f *testing.F) {
	f.Add(PDU{ID: PDUSearchAttributeRequest, Transaction: 1, Params: []byte{1, 2, 3}}.Marshal())
	f.Add([]byte{0x06, 0x00, 0x01, 0xff, 0xff})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, raw []byte) {
		p, err := ParsePDU(raw)
		if err != nil {
			return
		}
		p.Marshal()
	})
}
