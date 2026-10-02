// Package avrcp is the remote control: the buttons on the screen driving what is playing on the
// phone, and what is playing on the phone appearing on the screen.
//
// A2DP moves the audio and says nothing about it. Everything a player needs — the title, who it is
// by, how far through it is, and making a pause button do anything — is this profile, on a
// different L2CAP channel, in a different encoding, over a different transport. It is optional by
// the spec and expected by every phone.
//
// # WHICH END THIS IS
//
// A speaker is usually the controller: it has the buttons, the phone has the music. This device has
// both, so it is the controller for transport and metadata — play, pause, next, what is playing —
// and the target for absolute volume, so that the phone's own volume slider moves this one.
//
// THE THREE LAYERS
//
//	avctp      a transaction label and which profile this is for
//	av/c       a command type, a subunit, an opcode — a 1990s camcorder protocol
//	avrcp      what is actually wanted, either a button or a vendor-dependent pdu
//
// AV/C is there because AVRCP was defined on top of the AV/C Digital Interface Command Set, which
// was written for controlling camcorders over FireWire. That is why a pause button is addressed to
// a PANEL subunit, and why the useful half of this profile is carried as a vendor extension inside
// a command that was meant for something else.
//
// AVCTP is folded into this package rather than given its own. It is a layer of its own in the
// specs and about ninety lines here, with exactly one thing above it that will ever use it.
package avrcp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ErrShort is a buffer that does not hold what its header says.
var ErrShort = errors.New("avrcp: shorter than its header says")

// PID is the profile a message is for. AVRCP's, and the only one this answers.
const PID = 0x110e

// Which half of an exchange a message is.
const (
	MessageCommand  = 0
	MessageResponse = 1
)

// Whether a message stands alone or is a piece of a longer one.
const (
	PacketSingle   = 0
	PacketStart    = 1
	PacketContinue = 2
	PacketEnd      = 3
)

// avctpHeader is the label byte and the profile.
const avctpHeader = 3

// Transport is one AVCTP message: which transaction, which profile, and the AV/C frame inside.
type Transport struct {
	// Label pairs an answer with its question. Four bits, so it wraps at sixteen, and a controller
	// with several outstanding must not reuse one that has not been answered.
	Label byte

	// Type is command or response.
	Type byte

	// Invalid is a target saying it does not speak the profile that was asked for. Only responses
	// set it, and one that does carries no payload worth reading.
	Invalid bool

	Payload []byte
}

// ParseTransport reads one single-packet message.
//
// A fragment is refused rather than read short: half an AV/C frame parses into a plausible command
// for the wrong thing. Reassemble first.
func ParseTransport(buf []byte) (Transport, error) {
	if len(buf) < avctpHeader {
		return Transport{}, ErrShort
	}

	if packet := buf[0] >> 2 & 0x3; packet != PacketSingle {
		return Transport{}, fmt.Errorf("avrcp: packet type %d is a fragment, not a whole message",
			packet)
	}

	if pid := binary.BigEndian.Uint16(buf[1:]); pid != PID {
		return Transport{}, fmt.Errorf("avrcp: profile %#04x is not this one", pid)
	}

	return Transport{
		Label:   buf[0] >> 4,
		Type:    buf[0] >> 1 & 1,
		Invalid: buf[0]&1 != 0,
		Payload: buf[avctpHeader:],
	}, nil
}

// Marshal writes it as a single packet.
func (t Transport) Marshal() []byte {
	out := make([]byte, avctpHeader+len(t.Payload))

	out[0] = t.Label<<4 | PacketSingle<<2 | t.Type&1<<1
	if t.Invalid {
		out[0] |= 1
	}
	binary.BigEndian.PutUint16(out[1:], PID)
	copy(out[avctpHeader:], t.Payload)

	return out
}

// Reassembler puts a fragmented message back together.
//
// A metadata response carrying a long title does not fit an L2CAP packet, and this is one of the
// two ways the profile deals with that — the other being a continuation the controller asks for by
// name. Phones use both, so both have to work.
//
// One message in hand at a time: a run that never finishes is dropped when a new one starts.
type Reassembler struct {
	label   byte
	partial []byte
	left    int
	holding bool
}

// Push takes one packet and returns a whole message when there is one.
//
// A start packet says how many packets the message is in. That count is what says the run is
// complete; a run that ends early is dropped rather than handed over, because a truncated metadata
// response parses as a shorter one with different contents.
func (r *Reassembler) Push(buf []byte) ([]byte, error) {
	if len(buf) < avctpHeader {
		return nil, ErrShort
	}

	label := buf[0] >> 4
	packet := buf[0] >> 2 & 0x3

	switch packet {
	case PacketSingle:
		r.reset()
		return buf, nil

	case PacketStart:
		// A start packet puts the packet count between the label byte and the profile, so its
		// header is one byte longer than every other kind.
		if len(buf) < avctpHeader+1 {
			return nil, ErrShort
		}

		count := int(buf[1])
		if count < 2 {
			return nil, fmt.Errorf("avrcp: a start packet claiming %d packets", count)
		}

		r.label = label
		r.left = count - 1
		r.holding = true

		// Rebuilt as a single packet: the label byte with the type replaced, then the profile and
		// this packet's share of the payload.
		r.partial = append([]byte(nil), buf[0]&^byte(0x0c), buf[2], buf[3])
		r.partial = append(r.partial, buf[avctpHeader+1:]...)
		return nil, nil
	}

	// A continuation with nothing started, one for a different transaction, or one past the count
	// the start packet promised. Either way this side cannot say what the bytes belong to.
	if !r.holding || label != r.label || r.left <= 0 {
		r.reset()
		return nil, fmt.Errorf("avrcp: a %s packet for label %d with no run to join",
			packetName(packet), label)
	}

	r.partial = append(r.partial, buf[avctpHeader:]...)
	r.left--

	if packet != PacketEnd {
		return nil, nil
	}

	whole := r.partial
	short := r.left != 0
	r.reset()

	if short {
		return nil, fmt.Errorf("avrcp: a message ended %d packets early", r.left)
	}
	return whole, nil
}

func (r *Reassembler) reset() {
	r.partial = nil
	r.left = 0
	r.holding = false
}

func packetName(packet byte) string {
	switch packet {
	case PacketStart:
		return "start"
	case PacketContinue:
		return "continue"
	case PacketEnd:
		return "end"
	}
	return "single"
}
