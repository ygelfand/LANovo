package avdtp

import (
	"encoding/binary"
	"fmt"
)

// Media packets: the audio, which arrives on the transport channel rather than the signalling one.
//
// Everything else in this package is a request with an answer. This is the one direction that is
// neither — packets arrive as fast as the phone encodes them and nothing is sent back, so a mistake
// here is not a refusal but a stream that decodes to noise.
//
// Two headers sit in front of the frames. An RTP header, because A2DP carries audio the way
// everything else does, and then one byte of A2DP's own saying how many frames follow or which
// piece of one this is.

// rtpHeader is the fixed part, before any contributing sources or extension.
const rtpHeader = 12

// rtpVersion is the only version there has been.
const rtpVersion = 2

// Media is one packet off the transport channel.
type Media struct {
	// Sequence counts packets and wraps. A gap means a packet was lost, which is audible as a
	// click and not worth concealing here — a caller that cares can see the gap.
	Sequence uint16

	// Timestamp counts samples, not time, at whatever rate was configured.
	Timestamp uint32

	SSRC uint32

	// Fragmented says Payload is a piece of one frame rather than whole frames. Start and Last
	// mark the ends of the run; a fragment in the middle has neither.
	Fragmented bool
	Start      bool
	Last       bool

	// Count means two different things depending on Fragmented, which is the part of this header
	// worth reading twice. Whole packets: how many frames Payload holds. Fragments: how many
	// pieces of this frame are still to come, counting this one, so the last fragment says 1.
	Count int

	Payload []byte
}

// ParseMedia reads one packet.
func ParseMedia(buf []byte) (Media, error) {
	if len(buf) < rtpHeader+1 {
		return Media{}, ErrShort
	}

	if v := buf[0] >> 6; v != rtpVersion {
		return Media{}, fmt.Errorf("avdtp: rtp version %d, want %d", v, rtpVersion)
	}

	// Contributing sources and an extension are both legal and both rare. Skipping them by their
	// own lengths is cheaper than refusing a packet that is perfectly well formed.
	at := rtpHeader + int(buf[0]&0x0f)*4
	if buf[0]&0x10 != 0 {
		if len(buf) < at+4 {
			return Media{}, ErrShort
		}
		at += 4 + int(binary.BigEndian.Uint16(buf[at+2:]))*4
	}

	if len(buf) < at+1 {
		return Media{}, ErrShort
	}

	h := buf[at]
	return Media{
		Sequence:   binary.BigEndian.Uint16(buf[2:]),
		Timestamp:  binary.BigEndian.Uint32(buf[4:]),
		SSRC:       binary.BigEndian.Uint32(buf[8:]),
		Fragmented: h&0x80 != 0,
		Start:      h&0x40 != 0,
		Last:       h&0x20 != 0,
		Count:      int(h & 0x0f),
		Payload:    buf[at+1:],
	}, nil
}

// Marshal writes a packet. Only tests send these — a sink receives.
func (m Media) Marshal() []byte {
	out := make([]byte, rtpHeader+1+len(m.Payload))

	out[0] = rtpVersion << 6
	out[1] = payloadType
	binary.BigEndian.PutUint16(out[2:], m.Sequence)
	binary.BigEndian.PutUint32(out[4:], m.Timestamp)
	binary.BigEndian.PutUint32(out[8:], m.SSRC)

	var h byte
	if m.Fragmented {
		h |= 0x80
	}
	if m.Start {
		h |= 0x40
	}
	if m.Last {
		h |= 0x20
	}
	out[rtpHeader] = h | byte(m.Count)&0x0f

	copy(out[rtpHeader+1:], m.Payload)
	return out
}

// payloadType is the dynamic type A2DP conventionally uses. Nothing checks it on the way in,
// because phones do not agree on it and the transport channel carries nothing else anyway.
const payloadType = 96

// Reassembler puts fragmented frames back together.
//
// A frame is only split when it does not fit the transport channel's packet size, which at the
// bitpools a phone actually picks is uncommon — so this is the path that goes untested on a desk
// and then runs on somebody's phone. It keeps at most one frame in hand: a run that never finishes
// is dropped the moment a new one starts, rather than growing.
type Reassembler struct {
	partial []byte
	left    int
}

// Push takes a packet and returns whatever frames are now complete.
//
// A whole packet passes straight through. A fragment is held until its run finishes, and a run that
// does not add up is dropped — a frame assembled out of pieces of two different ones decodes to a
// burst of noise, which is worse through a speaker than a missing frame.
func (r *Reassembler) Push(m Media) []byte {
	if !m.Fragmented {
		r.reset()
		return m.Payload
	}

	if m.Start {
		// Start and last together is a frame fragmented into one piece, which is a contradiction
		// the spec allows and some encoders emit at the end of a run.
		if m.Last {
			r.reset()
			return m.Payload
		}

		r.partial = append([]byte(nil), m.Payload...)
		r.left = m.Count - 1
		return nil
	}

	// A fragment with no run started, or one more than the start packet promised. Either way this
	// side has lost track and cannot tell which frame the bytes belong to.
	if r.partial == nil || r.left <= 0 {
		r.reset()
		return nil
	}

	r.partial = append(r.partial, m.Payload...)
	r.left--

	if !m.Last {
		return nil
	}

	// The last fragment has to be the one the count said, or the frame has a hole in it.
	frame := r.partial
	short := r.left != 0
	r.reset()

	if short {
		return nil
	}
	return frame
}

func (r *Reassembler) reset() {
	r.partial = nil
	r.left = 0
}
