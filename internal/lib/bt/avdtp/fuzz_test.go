package avdtp

import "testing"

// Signalling and media off the transport channels. Everything here is a phone's bytes read before
// a stream is agreed, and the media path runs for every packet of audio after it.

// The signalling channel: a message, then whatever the signal says its body is. Chained, because a
// body is only read if the header let it through.
func FuzzParseMessage(f *testing.F) {
	f.Add([]byte{0x00, SignalDiscover})
	f.Add([]byte{0x02, SignalGetCapabilities, 0x01, 0x00})
	f.Add([]byte{0x00})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, raw []byte) {
		m, err := ParseMessage(raw)
		if err != nil {
			return
		}
		if len(m.Data) > len(raw) {
			t.Fatalf("a message carrying %d bytes out of %d", len(m.Data), len(raw))
		}

		// Both readings of the same body. Which one applies is a signal byte off the wire, so a
		// mislabelled message reaches the wrong one.
		ParseSEPs(m.Data)

		caps, err := ParseCapabilities(m.Data)
		if err != nil {
			return
		}
		for _, c := range caps {
			if len(c.Data) > len(raw) {
				t.Fatalf("a capability carrying %d bytes out of %d", len(c.Data), len(raw))
			}
			ParseSBC(c)
		}
	})
}

// The codec capability on its own, which is what decides how the decoder is set up.
func FuzzParseSBC(f *testing.F) {
	f.Add([]byte{0x21, 0x15, 0x02, 0x35})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseSBC(Capability{Category: CatMediaCodec, Data: data})
	})
}

// The media channel, packet by packet, through the reassembler that joins fragments. Fed in
// sequence rather than one at a time: a reassembler holds state between packets, and what it does
// with a fragment depends on what came before it.
func FuzzParseMedia(f *testing.F) {
	f.Add([]byte{0x80, 0x60, 0x00, 0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0x01, 0x9c})
	f.Add([]byte{0x80, 0x60, 0x00, 0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0x80})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, raw []byte) {
		m, err := ParseMedia(raw)
		if err != nil {
			return
		}
		if len(m.Payload) > len(raw) {
			t.Fatalf("a payload of %d bytes out of a packet of %d", len(m.Payload), len(raw))
		}

		// The same packet several times over, which is what a repeated or replayed fragment is.
		r := &Reassembler{}
		for range 3 {
			r.Push(m)
		}
	})
}
