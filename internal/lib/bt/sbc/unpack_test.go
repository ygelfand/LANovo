package sbc

import (
	"errors"
	"testing"
)

func TestTheBitReaderTakesBitsMostSignificantFirst(t *testing.T) {
	// 1 0 1 1 0 0 1 0 | 0 1 0 0 0 0 0 0, read three at a time and straddling the byte.
	r := &bitReader{buf: []byte{0b10110010, 0b01000000}}

	for _, want := range []uint32{0b101, 0b100, 0b100, 0b100} {
		got, err := r.read(3)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if got != want {
			t.Errorf("read %#b, want %#b", got, want)
		}
	}
}

// Running off the end is an error, not zeros. A frame claiming more than it holds would otherwise
// decode to silence and read as a quiet passage.
func TestReadingPastTheEndIsAnError(t *testing.T) {
	r := &bitReader{buf: []byte{0xff}}

	if _, err := r.read(8); err != nil {
		t.Fatalf("the first eight bits: %v", err)
	}
	if _, err := r.read(1); !errors.Is(err, ErrShort) {
		t.Errorf("reading past the end gave %v, want a short read", err)
	}
}

func TestAligningToAByte(t *testing.T) {
	r := &bitReader{buf: make([]byte, 4)}

	r.read(3)
	r.align()
	if r.at != 8 {
		t.Errorf("aligned to bit %d, want 8", r.at)
	}

	r.align()
	if r.at != 8 {
		t.Errorf("aligning twice moved to %d", r.at)
	}
}

// built is a whole frame with its check byte set, so Unpack will accept it.
func built(t *testing.T, mode, alloc, bitpool int, fill func(i int) byte) []byte {
	t.Helper()

	raw := header(44100, 16, mode, alloc, 8, bitpool)
	h, err := ParseHeader(raw)
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}

	frame := make([]byte, h.Length())
	copy(frame, raw)
	for i := headerBytes; i < len(frame); i++ {
		frame[i] = fill(i)
	}

	crc, err := CRC(h, frame)
	if err != nil {
		t.Fatalf("CRC: %v", err)
	}
	frame[3] = crc

	return frame
}

func TestUnpackingAFrame(t *testing.T) {
	frame := built(t, Stereo, Loudness, 53, func(i int) byte { return byte(i * 7) })

	f, err := Unpack(frame)
	if err != nil {
		t.Fatalf("Unpack: %v", err)
	}

	h := f.Header
	if len(f.Scale) != h.Subbands*h.Channels() {
		t.Errorf("%d scale factors, want %d", len(f.Scale), h.Subbands*h.Channels())
	}
	for i, sf := range f.Scale {
		if sf < 0 || sf > 15 {
			t.Errorf("scale factor %d is %d, outside the four bits it is coded in", i, sf)
		}
	}

	if len(f.Samples) != h.Blocks {
		t.Fatalf("%d blocks, want %d", len(f.Samples), h.Blocks)
	}
	for blk := range f.Samples {
		if len(f.Samples[blk]) != h.Channels() {
			t.Fatalf("block %d has %d channels", blk, len(f.Samples[blk]))
		}
		for ch := range f.Samples[blk] {
			if len(f.Samples[blk][ch]) != h.Subbands {
				t.Fatalf("block %d channel %d has %d subbands", blk, ch,
					len(f.Samples[blk][ch]))
			}
		}
	}
}

// A frame whose check byte does not match is refused rather than decoded. Decoding a corrupt frame
// produces a burst of noise through the speaker, which is worse than a dropped frame.
func TestACorruptFrameIsRefused(t *testing.T) {
	frame := built(t, Stereo, Loudness, 53, func(i int) byte { return byte(i) })
	frame[3] ^= 0xff

	if _, err := Unpack(frame); err == nil {
		t.Error("a frame with a wrong check byte was decoded")
	}
}

func TestAFrameThatHasNotAllArrived(t *testing.T) {
	frame := built(t, Stereo, Loudness, 53, func(i int) byte { return byte(i) })

	if _, err := Unpack(frame[:len(frame)-1]); !errors.Is(err, ErrShort) {
		t.Errorf("a short frame gave %v, want a short read", err)
	}
}

// Every mode has to unpack, since a phone chooses and we do not.
func TestEveryModeUnpacks(t *testing.T) {
	for _, mode := range []int{Mono, DualChannel, Stereo, JointStereo} {
		for _, alloc := range []int{Loudness, SNR} {
			bitpool := 53
			if mode == Mono || mode == DualChannel {
				bitpool = 32
			}

			frame := built(t, mode, alloc, bitpool, func(i int) byte { return byte(i * 5) })

			f, err := Unpack(frame)
			if err != nil {
				t.Fatalf("mode %d allocation %d: %v", mode, alloc, err)
			}
			if len(f.Samples) != f.Header.Blocks {
				t.Errorf("mode %d: %d blocks", mode, len(f.Samples))
			}
		}
	}
}

// Joint stereo carries nrof_subbands-1 flags in nrof_subbands bits: the last is reserved. Reading
// the reserved bit as a flag makes the top subband decode as sum and difference when it is not.
func TestJointStereoReadsOneFewerFlagThanBits(t *testing.T) {
	// Set every bit of the join field, which is the first byte after the header.
	frame := built(t, JointStereo, Loudness, 53, func(i int) byte {
		if i == headerBytes {
			return 0xff
		}
		return byte(i)
	})

	f, err := Unpack(frame)
	if err != nil {
		t.Fatalf("Unpack: %v", err)
	}

	// Seven flags for eight subbands, so the top one must not be joined however the bits read.
	if f.Join&(1<<7) != 0 {
		t.Error("the reserved bit was read as a join flag for the top subband")
	}
	if f.Join != 0x7f {
		t.Errorf("join came back %#x, want the seven flags set", f.Join)
	}
}

// Unjoining turns sum and difference back into two channels. Skipping it does not fail — it gives
// audio where one channel is the mix and the other the difference, which sounds like a broken
// stereo image rather than an error.
func TestUnjoiningRecoversTheTwoChannels(t *testing.T) {
	f := &Frame{
		Header: Header{Mode: JointStereo, Blocks: 1, Subbands: 2},
		Join:   0x01, // only the first subband is joined
		Samples: [][][]int32{{
			{100, 7},
			{40, 9},
		}},
	}

	f.unjoin()

	// Sum and difference: left becomes l+r, right becomes l-r.
	if f.Samples[0][0][0] != 140 {
		t.Errorf("the joined subband's left came back %d, want 140", f.Samples[0][0][0])
	}
	if f.Samples[0][1][0] != 60 {
		t.Errorf("the joined subband's right came back %d, want 60", f.Samples[0][1][0])
	}

	// The subband that was not joined is untouched.
	if f.Samples[0][0][1] != 7 || f.Samples[0][1][1] != 9 {
		t.Errorf("an unjoined subband changed: %d %d", f.Samples[0][0][1], f.Samples[0][1][1])
	}
}

// Nothing to unjoin in the other modes, and running it anyway would corrupt them.
func TestUnjoiningDoesNothingOutsideJointStereo(t *testing.T) {
	f := &Frame{
		Header:  Header{Mode: Stereo, Blocks: 1, Subbands: 1},
		Join:    0x01,
		Samples: [][][]int32{{{100}, {40}}},
	}

	f.unjoin()

	if f.Samples[0][0][0] != 100 || f.Samples[0][1][0] != 40 {
		t.Errorf("stereo was unjoined: %d %d", f.Samples[0][0][0], f.Samples[0][1][0])
	}
}

// A subband allocated no bits decodes to zero rather than being read from the stream, and reading
// it anyway would shift everything after it.
func TestASubbandWithNoBitsIsSilentAndConsumesNothing(t *testing.T) {
	// A tiny bitpool, so most subbands get nothing.
	frame := built(t, Mono, Loudness, 4, func(i int) byte { return byte(i * 3) })

	f, err := Unpack(frame)
	if err != nil {
		t.Fatalf("Unpack: %v", err)
	}

	bits := Allocate(f.Header, f.Scale)

	var silent int
	for sb := range f.Header.Subbands {
		if bits[0][sb] != 0 {
			continue
		}
		silent++
		for blk := range f.Header.Blocks {
			if f.Samples[blk][0][sb] != 0 {
				t.Fatalf("subband %d got no bits but decoded to %d",
					sb, f.Samples[blk][0][sb])
			}
		}
	}
	if silent == 0 {
		t.Skip("this bitpool gave every subband something, so there is nothing to check")
	}
}
