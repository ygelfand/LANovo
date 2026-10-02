// Package sbc decodes SBC, the sub-band codec every A2DP device has to support.
//
// A phone encodes to it and this device has to turn it back into samples the speaker can play.
// Pure arithmetic, no hardware, and the spec publishes test vectors — so unlike most of what this
// stack does, correctness here is checkable rather than argued about.
//
// Decode only. Nothing here encodes, because a sink never sends audio.
package sbc

import (
	"errors"
	"fmt"
)

// Syncword is the byte every frame starts with. Finding it is how a decoder picks the stream back
// up after a lost packet.
const Syncword = 0x9c

// ErrShort is a buffer that does not hold a whole frame yet.
var ErrShort = errors.New("sbc: shorter than a whole frame")

// How the two channels are carried.
const (
	Mono = iota
	DualChannel
	Stereo
	JointStereo
)

// How bits are shared between subbands.
const (
	Loudness = 0
	SNR      = 1
)

// headerBytes is the syncword, two packed bytes, the bitpool and the crc.
const headerBytes = 4

// rates, blocks and subbands, indexed by the bits that name them.
var (
	rates    = [4]int{16000, 32000, 44100, 48000}
	blocks   = [4]int{4, 8, 12, 16}
	subbands = [2]int{4, 8}
)

// Header is what a frame says about itself.
type Header struct {
	Rate       int
	Blocks     int
	Mode       int
	Allocation int
	Subbands   int
	Bitpool    int

	// CRC is what the frame claimed, for a caller that wants to check it.
	CRC byte
}

// Channels is how many the mode carries.
func (h Header) Channels() int {
	if h.Mode == Mono {
		return 1
	}
	return 2
}

// Joint reports whether the subbands may be shared between channels, which changes both the frame's
// length and what has to be read out of it.
func (h Header) Joint() bool { return h.Mode == JointStereo }

// Samples is how many samples per channel one frame decodes to.
func (h Header) Samples() int { return h.Blocks * h.Subbands }

// Length is the whole frame in bytes, header included.
//
// Three formulas rather than one, because the modes pack differently: dual channel spends bitpool
// per channel where stereo spends it across the pair, and joint stereo adds one bit per subband to
// say which are shared. Getting this wrong does not corrupt a frame, it loses the start of the next
// one — so the stream decodes to noise rather than failing.
func (h Header) Length() int {
	scale := 4 * h.Subbands * h.Channels() / 8

	var bits int
	switch h.Mode {
	case Mono, DualChannel:
		bits = h.Blocks * h.Channels() * h.Bitpool
	case Stereo:
		bits = h.Blocks * h.Bitpool
	default: // joint
		bits = h.Subbands + h.Blocks*h.Bitpool
	}

	return headerBytes + scale + (bits+7)/8
}

// ParseHeader reads a frame's header and works out how long the frame is.
//
// It does not check the CRC: that covers the scale factors as well as the header, so it needs the
// whole frame, and a caller reading a stream wants the length before it has one.
func ParseHeader(buf []byte) (Header, error) {
	if len(buf) < headerBytes {
		return Header{}, ErrShort
	}
	if buf[0] != Syncword {
		return Header{}, fmt.Errorf("sbc: %#02x is not the syncword", buf[0])
	}

	h := Header{
		Rate:       rates[buf[1]>>6&0x3],
		Blocks:     blocks[buf[1]>>4&0x3],
		Mode:       int(buf[1] >> 2 & 0x3),
		Allocation: int(buf[1] >> 1 & 0x1),
		Subbands:   subbands[buf[1]&0x1],
		Bitpool:    int(buf[2]),
		CRC:        buf[3],
	}

	// A bitpool of zero encodes nothing and a frame claiming it would have no payload, which is a
	// corrupt header rather than a silent frame.
	if h.Bitpool == 0 {
		return Header{}, errors.New("sbc: a frame with no bitpool")
	}

	// The spec caps it by mode and subbands. Past the cap the frame length runs away, and a decoder
	// that trusts it walks off the end of the stream.
	if max := h.maxBitpool(); h.Bitpool > max {
		return Header{}, fmt.Errorf("sbc: bitpool %d, above the %d this mode allows", h.Bitpool, max)
	}

	return h, nil
}

// maxBitpool is the ceiling for this mode and subband count.
func (h Header) maxBitpool() int {
	if h.Mode == Mono || h.Mode == DualChannel {
		return 16 * h.Subbands
	}
	return 32 * h.Subbands
}

// crcTable is the polynomial x^8 + x^4 + x^3 + x^2 + 1, which SBC uses over the header and scale
// factors. Built once rather than written out.
var crcTable = func() [256]byte {
	var t [256]byte
	for i := range 256 {
		c := byte(i)
		for range 8 {
			if c&0x80 != 0 {
				c = c<<1 ^ 0x1d
			} else {
				c <<= 1
			}
		}
		t[i] = c
	}
	return t
}()

// crcBits runs the check over a number of bits, which is needed because the run it covers does not
// end on a byte boundary in joint stereo.
func crcBits(buf []byte, bits int) byte {
	crc := byte(0x0f)

	whole := bits / 8
	for _, b := range buf[:whole] {
		crc = crcTable[crc^b]
	}

	// The leftover bits, most significant first.
	for i := range bits % 8 {
		bit := buf[whole] >> (7 - i) & 1

		top := crc >> 7 & 1
		crc <<= 1
		if top^bit != 0 {
			crc ^= 0x1d
		}
	}
	return crc
}

// CRC is what a whole frame's check byte should be.
//
// It covers bytes 1 and 2 of the header, then the join bits when there are any, then every scale
// factor — but not the syncword and not the check byte itself. The span ending mid-byte in joint
// stereo is why this counts bits rather than bytes.
func CRC(h Header, frame []byte) (byte, error) {
	if len(frame) < h.Length() {
		return 0, ErrShort
	}

	// Two header bytes, the join bits, and four bits per scale factor.
	bits := 16
	if h.Joint() {
		bits += h.Subbands
	}
	bits += 4 * h.Subbands * h.Channels()

	// Skipping the syncword and the check byte: the run starts at byte 1 and continues past byte 3.
	covered := make([]byte, 0, (bits+7)/8)
	covered = append(covered, frame[1], frame[2])
	covered = append(covered, frame[headerBytes:h.Length()]...)

	return crcBits(covered, bits), nil
}

// Valid reports whether a frame's check byte matches what its contents say it should be.
func Valid(h Header, frame []byte) bool {
	got, err := CRC(h, frame)
	return err == nil && got == h.CRC
}

// Find is the offset of the next frame in a buffer, for picking a stream back up after a gap.
//
// A syncword alone is nowhere near enough: 0x9c turns up in audio data constantly, and a run of
// bytes after one will parse as a header surprisingly often. Even the check byte only rules out
// 255 candidates in 256, which at the rate false syncwords appear is not rare enough — found once
// in a test with four bytes of junk in front of a real frame.
//
// So a candidate has to clear three hurdles: its header parses, its check byte matches, and the
// frame after it starts with a syncword too. The last is what makes this reliable, because two
// unrelated coincidences in a row is a different order of unlikely.
//
// At the end of a buffer there is no next frame to look at, and then the first two have to do.
func Find(buf []byte) (int, bool) {
	for i := range buf {
		if buf[i] != Syncword {
			continue
		}

		h, err := ParseHeader(buf[i:])
		if err != nil {
			continue
		}

		end := i + h.Length()
		if end > len(buf) || !Valid(h, buf[i:]) {
			continue
		}
		if end < len(buf) && buf[end] != Syncword {
			continue
		}
		return i, true
	}
	return 0, false
}
