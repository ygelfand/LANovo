package sbc

import "testing"

// A frame off the air, decoded. Every loop bound in here — blocks, subbands, channels, bitpool —
// is a header field a phone chose, and the bit reader walks a buffer whose length the same header
// decides. A panic is the daemon and the panel with it.

func FuzzParseHeader(f *testing.F) {
	f.Add([]byte{Syncword, 0x00, 0x20, 0x00})
	f.Add([]byte{Syncword, 0xff, 0xff, 0xff})
	f.Add([]byte{Syncword})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, raw []byte) {
		h, err := ParseHeader(raw)
		if err != nil {
			return
		}

		// A length that is not positive would have the reader spin or index backwards.
		if h.Length() <= 0 {
			t.Fatalf("a header claiming a frame of %d bytes: %+v", h.Length(), h)
		}
		if h.Channels() <= 0 || h.Samples() <= 0 {
			t.Fatalf("%d channels and %d samples: %+v", h.Channels(), h.Samples(), h)
		}
	})
}

func FuzzUnpack(f *testing.F) {
	f.Add([]byte{Syncword, 0x00, 0x20, 0x00, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{Syncword, 0x31, 0x2f, 0x00, 1, 2, 3, 4, 5, 6, 7, 8})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, raw []byte) {
		// As it arrives. Most of this is turned away at the header or the check byte, which is what
		// a real stream does when it loses sync.
		Unpack(raw)

		// And again with the check byte made to match, so the decoder is actually reached. Past
		// here every bound comes from the header.
		h, err := ParseHeader(raw)
		if err != nil || len(raw) < h.Length() || h.Length() < headerBytes {
			return
		}

		frame := append([]byte(nil), raw[:h.Length()]...)
		crc, err := CRC(h, frame)
		if err != nil {
			return
		}
		frame[3] = crc

		got, err := Unpack(frame)
		if err != nil {
			return
		}

		// The shape has to match what the header promised, since synthesis walks it by block,
		// channel and subband from the header rather than from the slices.
		if len(got.Samples) != h.Blocks {
			t.Fatalf("%d blocks of samples for a %d block header", len(got.Samples), h.Blocks)
		}
		for b, block := range got.Samples {
			if len(block) != h.Channels() {
				t.Fatalf("block %d has %d channels, the header says %d",
					b, len(block), h.Channels())
			}
			for ch, sub := range block {
				if len(sub) != h.Subbands {
					t.Fatalf("block %d channel %d has %d subbands, the header says %d",
						b, ch, len(sub), h.Subbands)
				}
			}
		}

		// Scale is laid out channel-major over the same two counts.
		if len(got.Scale) != h.Subbands*h.Channels() {
			t.Fatalf("%d scale factors for %d subbands across %d channels",
				len(got.Scale), h.Subbands, h.Channels())
		}

		// And through the filterbank, which sizes a ring and indexes it from the same header
		// fields. Twice over the same filter, because it keeps history between frames and the
		// second pass is the one that reads what the first wrote.
		var filter Filter
		for range 2 {
			audio, err := filter.Synthesize(got)
			if err != nil {
				break
			}
			if len(audio) != h.Channels() {
				t.Fatalf("%d channels of audio for a %d channel header", len(audio), h.Channels())
			}
			for ch, s := range audio {
				if len(s) != h.Blocks*h.Subbands {
					t.Fatalf("channel %d has %d samples, want %d blocks of %d subbands",
						ch, len(s), h.Blocks, h.Subbands)
				}
			}
		}
	})
}

// Find scans for a frame in a buffer, which is what a stream that lost sync calls on every byte.
func FuzzFind(f *testing.F) {
	f.Add([]byte{0x11, Syncword, 0x22, 0x33})
	f.Add([]byte{Syncword, Syncword, Syncword, Syncword})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, raw []byte) {
		at, ok := Find(raw)
		if !ok {
			return
		}
		if at < 0 || at >= len(raw) {
			t.Fatalf("a frame found at %d of %d bytes", at, len(raw))
		}
		if raw[at] != Syncword {
			t.Fatalf("a frame found at %d, where the byte is %#02x", at, raw[at])
		}
	})
}
