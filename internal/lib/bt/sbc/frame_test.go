package sbc

import (
	"errors"
	"testing"
)

// header builds the four header bytes for a set of choices, so a test can say what it means rather
// than a magic number.
func header(rate, block, mode, alloc, sub, bitpool int) []byte {
	var f, b, s byte
	for i, v := range rates {
		if v == rate {
			f = byte(i)
		}
	}
	for i, v := range blocks {
		if v == block {
			b = byte(i)
		}
	}
	for i, v := range subbands {
		if v == sub {
			s = byte(i)
		}
	}

	return []byte{
		Syncword,
		f<<6 | b<<4 | byte(mode)<<2 | byte(alloc)<<1 | s,
		byte(bitpool),
		0,
	}
}

func TestReadingAFrameHeader(t *testing.T) {
	got, err := ParseHeader(header(44100, 16, JointStereo, Loudness, 8, 53))
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}

	if got.Rate != 44100 {
		t.Errorf("rate came back %d", got.Rate)
	}
	if got.Blocks != 16 {
		t.Errorf("blocks came back %d", got.Blocks)
	}
	if got.Mode != JointStereo {
		t.Errorf("mode came back %d", got.Mode)
	}
	if got.Subbands != 8 {
		t.Errorf("subbands came back %d", got.Subbands)
	}
	if got.Bitpool != 53 {
		t.Errorf("bitpool came back %d", got.Bitpool)
	}
	if got.Channels() != 2 || !got.Joint() {
		t.Errorf("joint stereo came back as %d channels, joint=%v", got.Channels(), got.Joint())
	}
	if got.Samples() != 16*8 {
		t.Errorf("a frame decodes to %d samples, want 128", got.Samples())
	}
}

// Every combination has to round trip, because a field read one bit over reads the next one wrong
// and the frame length that follows is then nonsense.
func TestEveryHeaderCombinationReadsBack(t *testing.T) {
	for _, rate := range rates {
		for _, block := range blocks {
			for mode := range 4 {
				for alloc := range 2 {
					for _, sub := range subbands {
						got, err := ParseHeader(header(rate, block, mode, alloc, sub, 20))
						if err != nil {
							t.Fatalf("%d %d %d %d %d: %v", rate, block, mode, alloc, sub, err)
						}
						if got.Rate != rate || got.Blocks != block || got.Mode != mode ||
							got.Allocation != alloc || got.Subbands != sub {
							t.Errorf("wanted %d %d %d %d %d, got %+v",
								rate, block, mode, alloc, sub, got)
						}
					}
				}
			}
		}
	}
}

// The three modes pack differently, and a wrong length does not corrupt the frame — it loses the
// start of the next one, so the stream decodes to noise rather than failing.
func TestFrameLengthPerMode(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mode    int
		bitpool int
		want    int
	}{
		// 4 + 4*8*1/8 + ceil(16*1*32/8)
		{"mono", Mono, 32, 4 + 4 + 64},
		// 4 + 4*8*2/8 + ceil(16*2*32/8)
		{"dual", DualChannel, 32, 4 + 8 + 128},
		// 4 + 4*8*2/8 + ceil(16*53/8)
		{"stereo", Stereo, 53, 4 + 8 + 106},
		// 4 + 4*8*2/8 + ceil((8 + 16*53)/8)
		{"joint", JointStereo, 53, 4 + 8 + 107},
	} {
		h, err := ParseHeader(header(44100, 16, tc.mode, Loudness, 8, tc.bitpool))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := h.Length(); got != tc.want {
			t.Errorf("%s: frame length %d, want %d", tc.name, got, tc.want)
		}
	}
}

// A bitpool past the cap makes the frame length run away, and a decoder that trusts it walks off
// the end of the stream.
func TestABitpoolPastTheCapIsRefused(t *testing.T) {
	// Stereo with eight subbands allows 32*8 = 256, which does not fit a byte, so it cannot be
	// exceeded. Mono allows 16*8 = 128 and can.
	if _, err := ParseHeader(header(44100, 16, Mono, Loudness, 8, 129)); err == nil {
		t.Error("a mono bitpool of 129 was accepted, above the 128 allowed")
	}
	if _, err := ParseHeader(header(44100, 16, Mono, Loudness, 8, 128)); err != nil {
		t.Errorf("a mono bitpool of exactly 128 was refused: %v", err)
	}

	// Four subbands halves the ceiling.
	if _, err := ParseHeader(header(44100, 16, Mono, Loudness, 4, 65)); err == nil {
		t.Error("a bitpool of 65 was accepted with four subbands, above the 64 allowed")
	}
}

func TestAFrameWithNoBitpool(t *testing.T) {
	if _, err := ParseHeader(header(44100, 16, Stereo, Loudness, 8, 0)); err == nil {
		t.Error("a frame with no bitpool was accepted")
	}
}

func TestWhatIsNotAFrameHeader(t *testing.T) {
	if _, err := ParseHeader([]byte{0x00, 0x00, 0x10, 0x00}); err == nil {
		t.Error("a buffer with no syncword was read as a header")
	}
	if _, err := ParseHeader([]byte{Syncword, 0x00}); !errors.Is(err, ErrShort) {
		t.Errorf("half a header gave %v, want a short read", err)
	}
}

// The check byte covers the header and the scale factors but not the syncword or itself, and in
// joint stereo the span ends mid-byte — which is why it is counted in bits.
func TestTheCheckByteCoversTheRightSpan(t *testing.T) {
	raw := header(44100, 16, Stereo, Loudness, 8, 53)
	h, err := ParseHeader(raw)
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}

	frame := make([]byte, h.Length())
	copy(frame, raw)
	for i := headerBytes; i < len(frame); i++ {
		frame[i] = byte(i * 7)
	}

	want, err := CRC(h, frame)
	if err != nil {
		t.Fatalf("CRC: %v", err)
	}
	frame[3] = want
	h.CRC = want

	if !Valid(h, frame) {
		t.Fatal("a frame with its own check byte did not validate")
	}

	// The syncword is outside the span, so changing it must not change the check.
	moved := make([]byte, len(frame))
	copy(moved, frame)
	moved[0] = 0x00
	if got, _ := CRC(h, moved); got != want {
		t.Error("the check byte changed when the syncword did, so it is covering it")
	}

	// A scale factor is inside the span, so changing one must change the check.
	changed := make([]byte, len(frame))
	copy(changed, frame)
	changed[headerBytes] ^= 0xff
	if got, _ := CRC(h, changed); got == want {
		t.Error("the check byte did not change when a scale factor did")
	}

	// And a payload byte past the scale factors is outside it.
	late := make([]byte, len(frame))
	copy(late, frame)
	late[len(late)-1] ^= 0xff
	if got, _ := CRC(h, late); got != want {
		t.Error("the check byte changed when a payload byte did, so the span runs too long")
	}
}

// Joint stereo adds one bit per subband before the scale factors, so its span is not a whole number
// of bytes. A check that rounds to bytes covers the wrong run.
func TestTheCheckByteInJointStereo(t *testing.T) {
	raw := header(44100, 16, JointStereo, Loudness, 8, 53)
	h, _ := ParseHeader(raw)

	frame := make([]byte, h.Length())
	copy(frame, raw)
	for i := headerBytes; i < len(frame); i++ {
		frame[i] = byte(i * 13)
	}

	want, err := CRC(h, frame)
	if err != nil {
		t.Fatalf("CRC: %v", err)
	}

	// The last scale factor nibble is inside the span and the byte after it is not, which is the
	// boundary a byte-rounded check gets wrong.
	last := 2 + h.Subbands/8 + 4*h.Subbands*h.Channels()/8
	changed := make([]byte, len(frame))
	copy(changed, frame)
	changed[1+last-1] ^= 0x01

	if got, _ := CRC(h, changed); got == want {
		t.Error("changing the last covered bit did not change the check byte")
	}
}

func TestACheckOnAFrameThatHasNotArrived(t *testing.T) {
	h, _ := ParseHeader(header(44100, 16, Stereo, Loudness, 8, 53))

	if _, err := CRC(h, make([]byte, h.Length()-1)); !errors.Is(err, ErrShort) {
		t.Errorf("a short frame gave %v, want a short read", err)
	}
}

// good is one whole frame with a correct check byte.
func good(t *testing.T, seed byte) ([]byte, Header) {
	t.Helper()

	raw := header(44100, 16, Stereo, Loudness, 8, 53)
	h, err := ParseHeader(raw)
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}

	frame := make([]byte, h.Length())
	copy(frame, raw)
	for i := headerBytes; i < len(frame); i++ {
		frame[i] = byte(i)*3 + seed
	}

	crc, err := CRC(h, frame)
	if err != nil {
		t.Fatalf("CRC: %v", err)
	}
	frame[3] = crc
	h.CRC = crc

	return frame, h
}

// Picking the stream back up after a gap.
func TestFindingTheNextFrame(t *testing.T) {
	first, _ := good(t, 0)
	second, _ := good(t, 1)

	// Rubbish in front, including a syncword byte that starts nothing.
	junk := []byte{0x11, Syncword, 0x22, 0x33}

	buf := append(append(append([]byte{}, junk...), first...), second...)

	at, ok := Find(buf)
	if !ok {
		t.Fatal("the frame was not found")
	}
	if at != len(junk) {
		t.Errorf("found a frame at %d, want %d", at, len(junk))
	}
}

// The case that caught this: four bytes of junk in front of a real frame, where the junk's own
// syncword happened to parse as a header whose check byte collided. One in 256 is not rare enough
// at the rate false syncwords turn up, which is why a candidate also has to be followed by one.
func TestAFalseSyncwordIsNotMistakenForAFrame(t *testing.T) {
	first, _ := good(t, 0)
	second, _ := good(t, 1)

	junk := []byte{0x11, Syncword, 0x22, 0x33}
	buf := append(append(append([]byte{}, junk...), first...), second...)

	// The junk really does parse as a header, which is the whole point.
	if _, err := ParseHeader(buf[1:]); err != nil {
		t.Skip("the junk no longer parses as a header, so there is nothing to be fooled by")
	}

	if at, _ := Find(buf); at == 1 {
		t.Error("the false syncword was taken for a frame")
	}
}

// At the end of a buffer there is no following frame to confirm against, and a real last frame
// still has to be found.
func TestTheLastFrameIsStillFound(t *testing.T) {
	frame, _ := good(t, 7)

	at, ok := Find(frame)
	if !ok {
		t.Fatal("a lone frame was not found")
	}
	if at != 0 {
		t.Errorf("found it at %d, want the start", at)
	}
}

func TestFindingNothing(t *testing.T) {
	if _, ok := Find([]byte{1, 2, 3, 4, 5}); ok {
		t.Error("a frame was found in rubbish")
	}
	if _, ok := Find(nil); ok {
		t.Error("a frame was found in nothing")
	}
}
