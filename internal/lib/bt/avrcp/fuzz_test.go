package avrcp

import "testing"

// The remote control channel. A phone drives this one, and the metadata responses carry
// length-prefixed strings whose lengths are the sender's — track titles going straight to the
// panel.

// The whole stack on one buffer: transport, then the AV/C frame in it, then the profile PDU in
// that, then whatever the PDU id says its parameters are. Chained because each only runs on what
// the one before it allowed through.
func FuzzReceive(f *testing.F) {
	f.Add([]byte{0x00, 0x11, 0x0e, 0x00, 0x48, 0x00, 0x00, 0x19, 0x58})
	f.Add([]byte{0x00, 0x11, 0x0e, 0x01, 0x48, 0x00, 0x00, 0x19, 0x58,
		PDUGetElementAttributes, 0x00, 0x00, 0x00})
	f.Add([]byte{0x00})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, raw []byte) {
		tr, err := ParseTransport(raw)
		if err != nil {
			return
		}
		if len(tr.Payload) > len(raw) {
			t.Fatalf("a payload of %d bytes out of %d", len(tr.Payload), len(raw))
		}

		avc, err := ParseAVC(tr.Payload)
		if err != nil {
			return
		}
		if len(avc.Operands) > len(raw) {
			t.Fatalf("operands of %d bytes out of %d", len(avc.Operands), len(raw))
		}

		pdu, err := ParsePDU(avc.Operands)
		if err != nil {
			return
		}
		if len(pdu.Params) > len(raw) {
			t.Fatalf("parameters of %d bytes out of %d", len(pdu.Params), len(raw))
		}

		// Every reading of the same parameters. Which applies is the PDU id, off the wire, so a
		// mislabelled response reaches the wrong one.
		params(pdu.Params)
	})
}

// The parameter readings on their own, so they are reached with bodies the chained target would
// have to build a valid frame around first.
func FuzzParams(f *testing.F) {
	f.Add([]byte{0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x6a, 0x00, 0x04, 'S', 'o', 'n', 'g'})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})
	f.Add([]byte{0x00})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, raw []byte) { params(raw) })
}

func params(p []byte) {
	ParseElementAttributes(p)
	ParsePlayStatus(p)
	ParseNotification(p, true)
	ParseNotification(p, false)
	ParseCapabilities(p)
}
