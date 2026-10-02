package l2cap

import "testing"

// Everything a phone sends before it is anything to us. The signalling channel is open to whatever
// connects, and these run before any of it has been agreed, so a panic here is reachable by
// anything in radio range.

func FuzzParseFrame(f *testing.F) {
	f.Add(Frame{CID: CIDSignalling, Payload: []byte{1, 2, 3}}.Marshal())
	f.Add([]byte{0xff, 0xff, 0x40, 0x00})
	f.Add([]byte{0x00, 0x00})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, raw []byte) {
		fr, err := ParseFrame(raw)
		if err != nil {
			return
		}

		// The payload points into the buffer rather than being copied, so it has to stay inside it.
		if len(fr.Payload) > len(raw) {
			t.Fatalf("a payload of %d bytes out of a frame of %d", len(fr.Payload), len(raw))
		}
		if _, err := ParseFrame(fr.Marshal()); err != nil {
			t.Fatalf("a frame this parsed would not parse again: %v", err)
		}
	})
}

// The whole signalling path: a frame, the commands in it, and every request read out of each one.
// Chained because that is the order a connection arrives in, and a command only reaches the second
// parser if the first let it through.
func FuzzSignalling(f *testing.F) {
	f.Add([]byte{0x02, 0x01, 0x04, 0x00, 0x19, 0x00, 0x40, 0x00})
	f.Add([]byte{0x04, 0x01, 0x08, 0x00, 0x40, 0x00, 0x00, 0x00, 0x01, 0x02, 0xff, 0xff})
	f.Add([]byte{0x01, 0x01, 0xff, 0xff})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, payload []byte) {
		commands, err := ParseCommands(payload)
		if err != nil {
			return
		}

		for _, c := range commands {
			// Every one of these reads the same command, and which applies is decided by a code
			// byte off the wire. Running them all is what a mislabelled command does.
			ParseConnect(c)
			ParseConnected(c)
			ParseDisconnect(c)
			ParseConfigure(c)
			ParseConfigured(c)
		}
	})
}

// Configuration options are a list whose lengths come from the sender, inside a command whose
// length also does.
func FuzzParseOptions(f *testing.F) {
	f.Add([]byte{0x01, 0x02, 0xa0, 0x02})
	f.Add([]byte{0x01, 0xff})
	f.Add([]byte{0x00, 0x00, 0x00})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, raw []byte) {
		opts, err := ParseOptions(raw)
		if err != nil {
			return
		}

		for _, o := range opts {
			if len(o.Value) > len(raw) {
				t.Fatalf("an option carrying %d bytes out of %d", len(o.Value), len(raw))
			}
		}
	})
}
