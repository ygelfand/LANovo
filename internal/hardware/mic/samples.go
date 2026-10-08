package mic

import "encoding/binary"

func Samples(b []byte) []int16 {
	out := make([]int16, len(b)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	return out
}

func samplesOf(b []byte, bits int) []int16 {
	if bits != 32 {
		return Samples(b)
	}
	out := make([]int16, len(b)/4)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint32(b[i*4:]) >> 16)
	}
	return out
}

func Channel(samples []int16, ch int) []int16 {
	if ch < 0 || ch >= Channels {
		return nil
	}

	out := make([]int16, 0, len(samples)/Channels)
	for i := ch; i < len(samples); i += Channels {
		out = append(out, samples[i])
	}
	return out
}

func Peak(samples []int16) int {
	var peak int
	for _, s := range samples {
		v := int(s)
		if v < 0 {
			v = -v
		}
		if v > peak {
			peak = v
		}
	}
	return peak
}

func Identical(samples []int16) bool {
	if len(samples) < Channels {
		return false
	}
	for i := 0; i+1 < len(samples); i += Channels {
		if samples[i] != samples[i+1] {
			return false
		}
	}
	return true
}
