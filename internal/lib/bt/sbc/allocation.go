package sbc

// Bit allocation: how many bits each subband's samples were coded with.
//
// Nothing in the frame says this. Both ends run the same arithmetic over the scale factors and the
// bitpool and arrive at the same answer, and a decoder that computes it even slightly differently
// reads the sample bits at the wrong offsets — so the audio does not degrade, it becomes noise.
// That is what makes this worth writing out from the spec's own pseudocode rather than paraphrasing.

// offset4 and offset8 are the loudness tables, indexed by sampling frequency and then subband.
//
// They are what makes the loudness allocation spend fewer bits where the ear notices least, and
// they are simply constants from the specification.
var offset4 = [4][4]int{
	{-1, 0, 0, 0},
	{-2, 0, 0, 1},
	{-2, 0, 0, 1},
	{-2, 0, 0, 1},
}

var offset8 = [4][8]int{
	{-2, 0, 0, 0, 0, 0, 0, 1},
	{-3, 0, 0, 0, 0, 0, 1, 2},
	{-4, 0, 0, 0, 0, 0, 1, 2},
	{-4, 0, 0, 0, 0, 0, 1, 2},
}

// rateIndex is the position a rate has in the offset tables.
func rateIndex(rate int) int {
	for i, v := range rates {
		if v == rate {
			return i
		}
	}
	return 0
}

// offset is the loudness table entry for one subband.
func offset(rate, subbands, sb int) int {
	i := rateIndex(rate)
	if subbands == 4 {
		return offset4[i][sb]
	}
	return offset8[i][sb]
}

// bitneed is how many bits each subband is asking for, before the pool is shared out.
//
// SNR allocation asks for the scale factor itself. Loudness subtracts the table and halves what is
// left when positive, which is the whole of the psychoacoustics in this codec: a subband that is
// loud relative to what the ear expects there gets proportionally less.
func bitneed(h Header, scale []int, ch int) []int {
	out := make([]int, h.Subbands)

	for sb := range h.Subbands {
		sf := scale[ch*h.Subbands+sb]

		if h.Allocation == SNR {
			out[sb] = sf
			continue
		}

		if sf == 0 {
			out[sb] = -5
			continue
		}

		loud := sf - offset(h.Rate, h.Subbands, sb)
		if loud > 0 {
			loud /= 2
		}
		out[sb] = loud
	}
	return out
}

// share hands the bitpool out over a set of bitneeds.
//
// Straight from the specification's pseudocode, including the parts that look arbitrary. The
// bitslice loop finds the level at which the pool runs out, and the two passes after it spend
// whatever is left over — the first only on subbands already carrying something, the second on
// anything with room. Reordering those two changes the answer.
func share(need []int, bitpool int) []int {
	bits := make([]int, len(need))

	most := need[0]
	for _, v := range need[1:] {
		most = max(most, v)
	}

	bitcount, slicecount := 0, 0
	slice := most + 1

	for {
		slice--
		bitcount += slicecount
		slicecount = 0

		for _, v := range need {
			switch {
			case v > slice+1 && v < slice+16:
				slicecount++
			case v == slice+1:
				slicecount += 2
			}
		}

		if bitcount+slicecount >= bitpool {
			break
		}
		// Nothing more can be handed out: every subband is below the slice and lowering it further
		// changes nothing. Without this a pool larger than the signal needs never terminates.
		if slice < -16 {
			break
		}
	}

	// Landing exactly on the pool spends the slice rather than stopping above it. One off here
	// shifts every subband's width by one, and a decoder then reads each sample from the wrong
	// place in the frame.
	if bitcount+slicecount == bitpool {
		bitcount += slicecount
		slice--
	}

	for i, v := range need {
		if v < slice+2 {
			bits[i] = 0
			continue
		}
		bits[i] = min(v-slice, 16)
	}

	// The leftovers, in two passes. The first widens subbands that already carry something and
	// opens one that sat exactly on the boundary; the second fills anything with room left.
	for i := 0; i < len(need) && bitcount < bitpool; i++ {
		switch {
		case bits[i] >= 2 && bits[i] < 16:
			bits[i]++
			bitcount++
		case need[i] == slice+1 && bitpool > bitcount+1:
			bits[i] = 2
			bitcount += 2
		}
	}

	for i := 0; i < len(need) && bitcount < bitpool; i++ {
		if bits[i] < 16 {
			bits[i]++
			bitcount++
		}
	}
	return bits
}

// Allocate is how many bits each subband of each channel carries.
//
// Mono and dual channel share the pool per channel; stereo and joint share one pool across both,
// which is why the two are not the same function with a loop around it.
func Allocate(h Header, scale []int) [][]int {
	out := make([][]int, h.Channels())

	if h.Mode == Mono || h.Mode == DualChannel {
		for ch := range h.Channels() {
			out[ch] = share(bitneed(h, scale, ch), h.Bitpool)
		}
		return out
	}

	// One pool across both channels, and interleaved rather than laid end to end: the leftover
	// passes walk the set in order and stop when the pool runs out, so the order decides which
	// subbands get the last bits. The specification alternates channels and then steps the
	// subband, and spending them down one channel first leaves every sample after the first
	// disagreement being read from the wrong offset.
	left, right := bitneed(h, scale, 0), bitneed(h, scale, 1)

	both := make([]int, 0, len(left)+len(right))
	for sb := range h.Subbands {
		both = append(both, left[sb], right[sb])
	}

	bits := share(both, h.Bitpool)

	out[0] = make([]int, h.Subbands)
	out[1] = make([]int, h.Subbands)
	for sb := range h.Subbands {
		out[0][sb] = bits[sb*2]
		out[1][sb] = bits[sb*2+1]
	}
	return out
}
