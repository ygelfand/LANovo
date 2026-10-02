package sbc

import "fmt"

// The synthesis filterbank: subband samples in, audio out.
//
// It carries state between frames. Each output sample is a window over the last ten blocks of
// subband values, so a filterbank started fresh on every frame produces a click at every frame
// boundary — 128 times a second at the rates A2DP runs.
//
// Fixed point throughout, in the arrangement FFmpeg and bluez use, because the tables in
// synthesis_data.go are folded to suit it. The products are taken modulo 2^32 on purpose: the
// reference casts through unsigned for exactly that, and the overflow is part of the result rather
// than a fault to be avoided.

// The filterbank's history is ten blocks deep, which is where the 10 in every length here comes
// from: 8 phases of 10 for four subbands, 16 of 10 for eight.
const (
	history = 10

	// spill is how far past the end of the ring a read reaches. The wrap copies this many entries
	// from the start to the end so the ten reads are contiguous.
	spill = history - 1
)

// Filter turns frames into audio, keeping the filterbank's history between them.
//
// One per stream, not one per frame. It reshapes itself when the subband count or the channel count
// changes, which is what a phone reconfiguring mid-stream looks like.
type Filter struct {
	subbands int
	channels int

	// v is the ring of filtered values, one per channel, with spill entries of room past the end.
	v [][]int32

	// at is where each of the phases is writing, one set per channel.
	at [][]int
}

// Samples is one frame decoded: signed 16-bit, one slice per channel, Blocks*Subbands long.
type Samples [][]int16

// Synthesize turns a frame's subband samples into audio.
//
// The frame has to match the ones before it. A change in shape restarts the filterbank, which is
// correct — the history belongs to the old configuration — and audible as a click, which is why a
// sink should not be reconfiguring mid-song.
func (f *Filter) Synthesize(fr *Frame) (Samples, error) {
	h := fr.Header
	if h.Subbands != 4 && h.Subbands != 8 {
		return nil, fmt.Errorf("sbc: %d subbands, want 4 or 8", h.Subbands)
	}
	if len(fr.Samples) != h.Blocks {
		return nil, fmt.Errorf("sbc: %d blocks of samples for a %d block frame",
			len(fr.Samples), h.Blocks)
	}

	f.reshape(h)

	out := make(Samples, h.Channels())
	for ch := range out {
		out[ch] = make([]int16, h.Blocks*h.Subbands)
	}

	for blk := range h.Blocks {
		if len(fr.Samples[blk]) != h.Channels() {
			return nil, fmt.Errorf("sbc: block %d has %d channels, want %d",
				blk, len(fr.Samples[blk]), h.Channels())
		}

		for ch := range h.Channels() {
			if len(fr.Samples[blk][ch]) != h.Subbands {
				return nil, fmt.Errorf("sbc: block %d channel %d has %d subbands, want %d",
					blk, ch, len(fr.Samples[blk][ch]), h.Subbands)
			}
			f.block(fr.Samples[blk][ch], ch, out[ch][blk*h.Subbands:])
		}
	}
	return out, nil
}

// reshape starts the filterbank over when the stream is not the one it was set up for.
func (f *Filter) reshape(h Header) {
	if f.subbands == h.Subbands && f.channels == h.Channels() {
		return
	}

	f.subbands, f.channels = h.Subbands, h.Channels()

	phases := 2 * h.Subbands
	ring := phases * history

	f.v = make([][]int32, f.channels)
	f.at = make([][]int, f.channels)

	for ch := range f.v {
		f.v[ch] = make([]int32, ring+spill)
		f.at[ch] = make([]int, phases)

		// One past where the first write goes: every phase steps back before it writes, so this
		// lands the first one at the end of the ring.
		for i := range f.at[ch] {
			f.at[ch][i] = history * (i + 1)
		}
	}
}

// block runs one block of one channel through the filterbank, writing subbands samples.
func (f *Filter) block(in []int32, ch int, out []int16) {
	matrix, m0, m1 := synMatrix4, proto4m0, proto4m1
	if f.subbands == 8 {
		matrix, m0, m1 = synMatrix8, proto8m0, proto8m1
	}

	v, at := f.v[ch], f.at[ch]
	phases := len(at)
	ring := phases * history

	// Step every phase back one and put this block's filtered value there.
	for i := range phases {
		at[i]--
		if at[i] < 0 {
			at[i] = ring - 1

			// The reads below run ten entries past a phase, so the start of the ring is mirrored
			// past its end rather than the reads being made to wrap.
			copy(v[ring:], v[:spill])
		}

		var acc uint32
		for sb, s := range in {
			acc += uint32(matrix[i][sb]) * uint32(s)
		}
		v[at[i]] = int32(acc) >> 15
	}

	// Each output sample is ten windowed values, taken alternately from a phase and its opposite.
	for i := range f.subbands {
		lo, hi := at[i], at[(i+f.subbands)&(phases-1)]

		var acc uint32
		for j := range history / 2 {
			acc += uint32(v[lo+2*j]) * uint32(m0[i*(history/2)+j])
			acc += uint32(v[hi+2*j+1]) * uint32(m1[i*(history/2)+j])
		}
		out[i] = clip16(int32(acc) >> 15)
	}
}

// clip16 saturates rather than wrapping. A sample past the end of the range is loud; the same
// sample wrapped is the opposite sign at full scale, which is a crack.
func clip16(v int32) int16 {
	switch {
	case v > 0x7fff:
		return 0x7fff
	case v < -0x8000:
		return -0x8000
	}
	return int16(v)
}
