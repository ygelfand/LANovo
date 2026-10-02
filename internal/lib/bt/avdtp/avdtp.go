// Package avdtp speaks Audio/Video Distribution Transport Protocol, which is the conversation
// between a phone and this device about how audio will be sent.
//
// The shape of it: the phone discovers what stream endpoints exist, asks what one can do, picks a
// configuration, opens it, and starts it. Six messages and the music plays. Everything else in the
// protocol is for reconfiguring, suspending and tearing down.
//
// This side is the acceptor throughout — a sink is discovered and configured, it does not go
// looking. Bytes in, bytes out, on an L2CAP channel somebody else opened.
package avdtp

import (
	"errors"
	"fmt"
)

// ErrShort is a message that does not hold what its header says.
var ErrShort = errors.New("avdtp: shorter than its header says")

// What a message is asking for.
const (
	SignalDiscover        = 0x01
	SignalGetCapabilities = 0x02
	SignalSetConfig       = 0x03
	SignalGetConfig       = 0x04
	SignalReconfigure     = 0x05
	SignalOpen            = 0x06
	SignalStart           = 0x07
	SignalClose           = 0x08
	SignalSuspend         = 0x09
	SignalAbort           = 0x0a
	SignalSecurity        = 0x0b
	SignalGetAllCaps      = 0x0c
	SignalDelayReport     = 0x0d
)

// Which half of an exchange a message is.
const (
	Command        = 0
	GeneralReject  = 1
	ResponseAccept = 2
	ResponseReject = 3
)

// Whether a message stands alone or is part of a longer one.
const (
	PacketSingle   = 0
	PacketStart    = 1
	PacketContinue = 2
	PacketEnd      = 3
)

// Message is one AVDTP signalling message.
type Message struct {
	// Label pairs an answer with its question. Four bits, so it wraps at sixteen.
	Label byte

	Type   byte
	Signal byte
	Data   []byte
}

// ParseMessage reads one message.
//
// Single packet only. A message longer than the channel's MTU is split, and at the sizes A2DP
// signalling runs to — a capabilities response is tens of bytes against an MTU of hundreds — that
// does not happen. It is detected and refused rather than mis-read, because a silently truncated
// capability list is a stream configured wrong.
func ParseMessage(buf []byte) (Message, error) {
	if len(buf) < 2 {
		return Message{}, ErrShort
	}

	packet := buf[0] >> 2 & 0x3
	if packet != PacketSingle {
		return Message{}, fmt.Errorf("avdtp: a fragmented message, packet type %d", packet)
	}

	return Message{
		Label:  buf[0] >> 4,
		Type:   buf[0] & 0x3,
		Signal: buf[1] & 0x3f,
		Data:   buf[2:],
	}, nil
}

// Marshal writes it.
func (m Message) Marshal() []byte {
	out := make([]byte, 2+len(m.Data))
	out[0] = m.Label<<4 | PacketSingle<<2 | m.Type&0x3
	out[1] = m.Signal & 0x3f
	copy(out[2:], m.Data)
	return out
}

// Accept is the answer to a command that worked.
func (m Message) Accept(data ...byte) Message {
	return Message{Label: m.Label, Type: ResponseAccept, Signal: m.Signal, Data: data}
}

// Reject is the answer to one that did not, carrying why.
//
// The error code goes in the payload, and for the configuration signals it is preceded by the
// category that was the problem — a phone that is told which capability it got wrong can try again,
// where one told only "no" gives up.
func (m Message) Reject(code byte) Message {
	return Message{Label: m.Label, Type: ResponseReject, Signal: m.Signal, Data: []byte{code}}
}

// RejectConfig is the refusal the configuration signals use, which names the category first.
func (m Message) RejectConfig(category, code byte) Message {
	return Message{Label: m.Label, Type: ResponseReject, Signal: m.Signal,
		Data: []byte{category, code}}
}

// Why a command was refused. Only the ones a sink has reason to send.
const (
	ErrorBadHeaderFormat   = 0x01
	ErrorBadLength         = 0x11
	ErrorBadSEID           = 0x12
	ErrorSEPInUse          = 0x13
	ErrorSEPNotInUse       = 0x14
	ErrorBadCategory       = 0x17
	ErrorBadPayloadFormat  = 0x18
	ErrorNotSupported      = 0x19
	ErrorBadState          = 0x31
	ErrorUnsupportedConfig = 0x29
)

// What kind of media an endpoint carries.
const (
	MediaAudio      = 0x00
	MediaVideo      = 0x01
	MediaMultimedia = 0x02
)

// Which end of a stream an endpoint is.
const (
	Source = 0
	Sink   = 1
)

// SEP is a stream endpoint: one thing on this device that audio can be sent to.
//
// A sink advertises one. The phone discovers it, asks what it can do, and configures it.
type SEP struct {
	// SEID identifies it, six bits, and may not be zero — zero is reserved and a phone that is
	// offered it has nothing to address.
	SEID byte

	InUse bool
	Media byte
	TSEP  byte
}

// sepBytes is what one endpoint takes in a discover response.
const sepBytes = 2

// Marshal writes the two bytes a discover response carries per endpoint.
func (s SEP) Marshal() []byte {
	first := s.SEID << 2
	if s.InUse {
		first |= 1 << 1
	}
	return []byte{first, s.Media<<4 | s.TSEP<<3}
}

// ParseSEPs reads the endpoint list out of a discover response.
func ParseSEPs(data []byte) ([]SEP, error) {
	if len(data)%sepBytes != 0 {
		return nil, fmt.Errorf("avdtp: %d bytes of endpoints, not a whole number of them", len(data))
	}

	out := make([]SEP, 0, len(data)/sepBytes)
	for i := 0; i < len(data); i += sepBytes {
		out = append(out, SEP{
			SEID:  data[i] >> 2,
			InUse: data[i]&(1<<1) != 0,
			Media: data[i+1] >> 4,
			TSEP:  data[i+1] >> 3 & 1,
		})
	}
	return out, nil
}

// SEID reads the endpoint a command is addressed to, which most of them carry as their first byte
// in the top six bits.
func SEID(data []byte) (byte, bool) {
	if len(data) < 1 {
		return 0, false
	}
	return data[0] >> 2, true
}

// AddressTo writes a command's endpoint byte.
func AddressTo(seid byte) byte { return seid << 2 }
