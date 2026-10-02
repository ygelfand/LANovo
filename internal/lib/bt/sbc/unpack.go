package sbc

import "fmt"

// Unpacking a frame: the scale factors, the coded samples, and turning those back into subband
// values. What comes out still has to go through the synthesis filterbank to be audio.
//
// Checked against ffmpeg's libavcodec/sbcdec.c, which descends from the bluez reference. Two things
// in here are easy to get subtly wrong and were confirmed rather than assumed: joint stereo
// contributes nrof_subbands bits to the CRC while carrying only nrof_subbands-1 join flags, and the
// dequantisation is exact integer arithmetic that must not be rewritten in floating point.

// bits reads a bitstream most significant bit first, which is how SBC is packed.
type bitReader struct {
	buf []byte
	at  int // in bits
}

// read takes n bits. Running off the end is an error rather than zeros: a frame that says it holds
// more than it does would otherwise decode to silence and look like a quiet passage.
func (r *bitReader) read(n int) (uint32, error) {
	if r.at+n > len(r.buf)*8 {
		return 0, ErrShort
	}

	var v uint32
	for range n {
		v <<= 1
		if r.buf[r.at>>3]>>(7-r.at&7)&1 != 0 {
			v |= 1
		}
		r.at++
	}
	return v, nil
}

// align moves to the next byte boundary, which is where a frame ends.
func (r *bitReader) align() {
	if r.at&7 != 0 {
		r.at += 8 - r.at&7
	}
}

// Frame is one decoded frame, before synthesis.
type Frame struct {
	Header Header

	// Join says which subbands have their two channels carried as sum and difference. Only joint
	// stereo sets any.
	Join uint32

	// Scale is the exponent per channel and subband, laid out channel-major.
	Scale []int

	// Samples are the subband values, indexed block, channel, subband.
	Samples [][][]int32
}

// extraBits is the headroom the reference decoder carries through dequantisation so the synthesis
// filterbank has precision to work with. Keeping the same number keeps the output identical.
const extraBits = 2

// Unpack reads a whole frame.
func Unpack(frame []byte) (*Frame, error) {
	h, err := ParseHeader(frame)
	if err != nil {
		return nil, err
	}
	if len(frame) < h.Length() {
		return nil, ErrShort
	}
	if !Valid(h, frame) {
		return nil, fmt.Errorf("sbc: the frame's check byte does not match its contents")
	}

	f := &Frame{Header: h}
	r := &bitReader{buf: frame[:h.Length()], at: headerBytes * 8}

	// Joint stereo says which subbands are carried as sum and difference. There are
	// nrof_subbands-1 flags and nrof_subbands bits: the last is reserved and read past.
	if h.Joint() {
		for sb := range h.Subbands - 1 {
			v, err := r.read(1)
			if err != nil {
				return nil, err
			}
			if v != 0 {
				f.Join |= 1 << sb
			}
		}
		if _, err := r.read(1); err != nil {
			return nil, err
		}
	}

	f.Scale = make([]int, h.Subbands*h.Channels())
	for i := range f.Scale {
		v, err := r.read(4)
		if err != nil {
			return nil, err
		}
		f.Scale[i] = int(v)
	}

	bits := Allocate(h, f.Scale)

	if err := f.readSamples(r, bits); err != nil {
		return nil, err
	}
	f.unjoin()

	return f, nil
}

// readSamples pulls the coded values out and turns them back into subband samples.
//
// The dequantisation is the reference's exact integer expression and is left in that shape on
// purpose. It looks like it could be simplified or done in floating point; both change the result
// in the low bits, and the low bits are what the test vectors check.
func (f *Frame) readSamples(r *bitReader, bits [][]int) error {
	h := f.Header

	levels := make([][]int64, h.Channels())
	for ch := range levels {
		levels[ch] = make([]int64, h.Subbands)
		for sb := range h.Subbands {
			levels[ch][sb] = int64(1)<<uint(bits[ch][sb]) - 1
		}
	}

	f.Samples = make([][][]int32, h.Blocks)
	for blk := range h.Blocks {
		f.Samples[blk] = make([][]int32, h.Channels())

		for ch := range h.Channels() {
			f.Samples[blk][ch] = make([]int32, h.Subbands)

			for sb := range h.Subbands {
				if levels[ch][sb] == 0 {
					continue
				}

				raw, err := r.read(bits[ch][sb])
				if err != nil {
					return err
				}

				shift := uint(f.Scale[ch*h.Subbands+sb] + 1 + extraBits)
				v := (int64(raw)<<1 | 1) << shift / levels[ch][sb]

				f.Samples[blk][ch][sb] = int32(v - 1<<shift)
			}
		}
	}

	r.align()
	return nil
}

// unjoin turns the subbands that were sent as sum and difference back into two channels.
//
// Joint stereo is the default a phone picks, so this runs on almost every real stream. Skipping it
// does not fail — it produces audio where one channel is the mix and the other is the difference,
// which sounds like a broken stereo image rather than an error.
func (f *Frame) unjoin() {
	if !f.Header.Joint() {
		return
	}

	for blk := range f.Header.Blocks {
		for sb := range f.Header.Subbands {
			if f.Join&(1<<sb) == 0 {
				continue
			}

			left := f.Samples[blk][0][sb]
			right := f.Samples[blk][1][sb]

			f.Samples[blk][0][sb] = left + right
			f.Samples[blk][1][sb] = left - right
		}
	}
}
