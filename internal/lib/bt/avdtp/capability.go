package avdtp

import "fmt"

// Capabilities are what an endpoint can do, and what a phone picks from.
//
// A sink advertises two: that it can carry media at all, and which codec settings it accepts. The
// phone answers with one configuration chosen out of those, and the pair have to agree exactly —
// a configuration naming something that was never offered is the refusal every stack sends.

// The categories a capability can be.
const (
	CatMediaTransport = 0x01
	CatReporting      = 0x02
	CatRecovery       = 0x03
	CatContentProtect = 0x04
	CatHeaderCompress = 0x05
	CatMultiplexing   = 0x06
	CatMediaCodec     = 0x07
	CatDelayReporting = 0x08
)

// Capability is one entry in the list.
type Capability struct {
	Category byte
	Data     []byte
}

// Marshal writes the category, the length, and the body.
func (c Capability) Marshal() []byte {
	out := make([]byte, 2+len(c.Data))
	out[0] = c.Category
	out[1] = byte(len(c.Data))
	copy(out[2:], c.Data)
	return out
}

// ParseCapabilities reads a run of them.
func ParseCapabilities(buf []byte) ([]Capability, error) {
	var out []Capability

	for len(buf) > 0 {
		if len(buf) < 2 {
			return nil, fmt.Errorf("avdtp: %d bytes left over, less than a capability header",
				len(buf))
		}

		n := int(buf[1])
		if len(buf) < 2+n {
			return nil, fmt.Errorf("avdtp: a capability says %d bytes and %d are left",
				n, len(buf)-2)
		}

		out = append(out, Capability{Category: buf[0], Data: buf[2 : 2+n]})
		buf = buf[2+n:]
	}
	return out, nil
}

// MarshalCapabilities writes a list.
func MarshalCapabilities(caps []Capability) []byte {
	var out []byte
	for _, c := range caps {
		out = append(out, c.Marshal()...)
	}
	return out
}

// Find is the capability of a category, and whether it is there.
func Find(caps []Capability, category byte) (Capability, bool) {
	for _, c := range caps {
		if c.Category == category {
			return c, true
		}
	}
	return Capability{}, false
}

// CodecSBC is the one codec every A2DP device has to support, and the only one this needs.
const CodecSBC = 0x00

// The sampling rates an SBC capability can offer, as the bits they are written with.
const (
	Rate16000 = 1 << 7
	Rate32000 = 1 << 6
	Rate44100 = 1 << 5
	Rate48000 = 1 << 4
)

// How the two channels are carried. Joint stereo is what a phone picks when it can, because it
// spends the fewest bits for the same result.
const (
	ChannelMono        = 1 << 3
	ChannelDualChannel = 1 << 2
	ChannelStereo      = 1 << 1
	ChannelJointStereo = 1 << 0
)

// How many samples go in a block.
const (
	Block4  = 1 << 7
	Block8  = 1 << 6
	Block12 = 1 << 5
	Block16 = 1 << 4
)

// How many subbands the spectrum is split into.
const (
	Subbands4 = 1 << 3
	Subbands8 = 1 << 2
)

// How bits are shared out between subbands.
const (
	AllocationSNR      = 1 << 1
	AllocationLoudness = 1 << 0
)

// SBC is the codec capability, unpacked.
//
// Offered, every field is a set of bits saying what is acceptable. Configured, each has exactly one
// bit set — the phone's choice. That is the same four bytes meaning two different things depending
// on which direction it travelled, which is worth knowing before reading one.
type SBC struct {
	Rates      byte
	Channels   byte
	Blocks     byte
	Subbands   byte
	Allocation byte

	MinBitpool byte
	MaxBitpool byte
}

// sbcBytes is the codec-specific part: two packed bytes and the two bitpool bounds.
const sbcBytes = 4

// Capability wraps it as a media codec capability, which is how it goes on the wire.
func (s SBC) Capability() Capability {
	return Capability{
		Category: CatMediaCodec,
		Data: []byte{
			MediaAudio << 4,
			CodecSBC,
			s.Rates | s.Channels,
			s.Blocks | s.Subbands | s.Allocation,
			s.MinBitpool,
			s.MaxBitpool,
		},
	}
}

// ParseSBC reads a media codec capability as SBC.
func ParseSBC(c Capability) (SBC, error) {
	if c.Category != CatMediaCodec {
		return SBC{}, fmt.Errorf("avdtp: category %#x is not a media codec", c.Category)
	}
	// Media type, codec type, then the codec's own bytes.
	if len(c.Data) < 2+sbcBytes {
		return SBC{}, ErrShort
	}
	if c.Data[1] != CodecSBC {
		return SBC{}, fmt.Errorf("avdtp: codec %#x is not sbc", c.Data[1])
	}

	return SBC{
		Rates:      c.Data[2] & 0xf0,
		Channels:   c.Data[2] & 0x0f,
		Blocks:     c.Data[3] & 0xf0,
		Subbands:   c.Data[3] & 0x0c,
		Allocation: c.Data[3] & 0x03,
		MinBitpool: c.Data[4],
		MaxBitpool: c.Data[5],
	}, nil
}

// SinkSBC is what this device offers: everything SBC allows, so a phone can pick whatever suits it.
//
// Offering the lot rather than a preference. A sink that advertises narrowly makes the phone encode
// to something it is worse at, and the bitpool bounds are the spec's own defaults.
func SinkSBC() SBC {
	return SBC{
		Rates:      Rate44100 | Rate48000,
		Channels:   ChannelMono | ChannelDualChannel | ChannelStereo | ChannelJointStereo,
		Blocks:     Block4 | Block8 | Block12 | Block16,
		Subbands:   Subbands4 | Subbands8,
		Allocation: AllocationSNR | AllocationLoudness,
		MinBitpool: 2,
		MaxBitpool: 53,
	}
}

// SinkCapabilities is the whole list an endpoint answers a GetCapabilities with.
func SinkCapabilities() []Capability {
	return []Capability{
		{Category: CatMediaTransport},
		SinkSBC().Capability(),
	}
}

// one reports whether exactly one bit is set, which is what a configuration must have where an
// offer may have several.
func one(v byte) bool { return v != 0 && v&(v-1) == 0 }

// Chosen reports whether this is a configuration rather than an offer: exactly one option picked in
// each field.
//
// Worth checking before acting on one. A phone that leaves two rates set has not chosen, and
// guessing which it meant is how a stream runs at the wrong speed and sounds slow.
func (s SBC) Chosen() bool {
	return one(s.Rates) && one(s.Channels) && one(s.Blocks) && one(s.Subbands) && one(s.Allocation)
}

// Within reports whether every option this configuration picked was offered.
//
// The refusal every stack sends is for a configuration naming something never advertised, so this
// is the check before accepting one.
func (s SBC) Within(offer SBC) bool {
	return s.Rates&offer.Rates == s.Rates &&
		s.Channels&offer.Channels == s.Channels &&
		s.Blocks&offer.Blocks == s.Blocks &&
		s.Subbands&offer.Subbands == s.Subbands &&
		s.Allocation&offer.Allocation == s.Allocation &&
		s.MaxBitpool <= offer.MaxBitpool &&
		s.MinBitpool >= offer.MinBitpool
}

// Rate is the sampling rate a configuration picked, in hertz.
func (s SBC) Rate() (int, bool) {
	switch s.Rates {
	case Rate16000:
		return 16000, true
	case Rate32000:
		return 32000, true
	case Rate44100:
		return 44100, true
	case Rate48000:
		return 48000, true
	}
	return 0, false
}
