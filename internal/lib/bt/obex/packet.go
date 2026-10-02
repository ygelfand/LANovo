package obex

import (
	"encoding/binary"
	"fmt"
)

// Packets. Everything is an opcode, a length that counts itself, and then headers — except connect,
// which puts four fixed bytes in front of its headers, and its answer, which does the same.

// packetHeader is the opcode and the length.
const packetHeader = 3

// Version is the protocol this speaks, as the wire writes it: one point zero.
const Version = 0x10

// Packet is one request or one answer.
type Packet struct {
	// Code is the opcode on the way out and the response code on the way back.
	Code byte

	Headers []Header

	// Version, Flags and MTU are only in a connect and its answer, and zero elsewhere.
	Version byte
	Flags   byte
	MTU     uint16
}

// connects reports whether a code carries the four extra bytes.
func connects(code byte) bool { return code == OpConnect }

// Connect opens a session, saying who it wants to talk to.
//
// The target identifies which service on the far end, since one channel may front several. Cover
// art has a uuid of its own, and a target that does not recognise it refuses rather than guessing.
func Connect(mtu uint16, target []byte) Packet {
	p := Packet{Code: OpConnect, Version: Version, MTU: mtu}
	if len(target) > 0 {
		p.Headers = append(p.Headers, Bytes(HeaderTarget, target))
	}
	return p
}

// Get asks for an object. Final because everything this sends fits in one packet.
func Get(connection uint32, headers ...Header) Packet {
	return Packet{
		Code:    OpGet | Final,
		Headers: append([]Header{Uint32(HeaderConnection, connection)}, headers...),
	}
}

// More asks for the rest of a body, which is the same get with nothing but the session on it.
//
// A target that answered Continue is holding the remainder and will not send it unasked. Repeating
// the name and type instead would start the fetch again.
func More(connection uint32) Packet { return Get(connection) }

// Disconnect closes the session.
func Disconnect(connection uint32) Packet {
	return Packet{Code: OpDisconnect, Headers: []Header{Uint32(HeaderConnection, connection)}}
}

// Marshal writes it.
func (p Packet) Marshal() []byte {
	var body []byte

	if connects(p.Code) {
		body = []byte{p.Version, p.Flags, 0, 0}
		binary.BigEndian.PutUint16(body[2:], p.MTU)
	}

	for _, h := range p.Headers {
		body = append(body, h.Marshal()...)
	}

	out := make([]byte, packetHeader, packetHeader+len(body))
	out[0] = p.Code
	binary.BigEndian.PutUint16(out[1:], uint16(packetHeader+len(body)))

	return append(out, body...)
}

// ParseAnswer reads a response.
//
// connected says whether this answers a connect, because only then are the four fixed bytes there.
// Nothing in the response itself distinguishes the two, which is a wart of the protocol rather than
// of this.
func ParseAnswer(buf []byte, connected bool) (Packet, error) {
	if len(buf) < packetHeader {
		return Packet{}, ErrShort
	}

	n := int(binary.BigEndian.Uint16(buf[1:]))
	if n < packetHeader || len(buf) < n {
		return Packet{}, fmt.Errorf("obex: a packet says %d bytes and %d arrived", n, len(buf))
	}

	p := Packet{Code: buf[0]}
	body := buf[packetHeader:n]

	if connected {
		if len(body) < 4 {
			return p, ErrShort
		}
		p.Version, p.Flags = body[0], body[1]
		p.MTU = binary.BigEndian.Uint16(body[2:])
		body = body[4:]
	}

	headers, err := ParseHeaders(body)
	p.Headers = headers
	return p, err
}

// Connection is the session identifier the far end handed out, which every later request carries.
func (p Packet) Connection() (uint32, bool) {
	h, ok := Find(p.Headers, HeaderConnection)
	if !ok {
		return 0, false
	}
	return h.Uint32()
}

// Body is the object's bytes in this packet, and whether that was the last of them.
//
// A long object arrives over several answers: each carries a Body and the final one an EndOfBody.
// A short one is a single answer carrying only EndOfBody.
func (p Packet) Body() (data []byte, done bool) {
	for _, h := range p.Headers {
		switch h.ID {
		case HeaderBody:
			data = append(data, h.Value...)
		case HeaderEndOfBody:
			data = append(data, h.Value...)
			done = true
		}
	}
	return data, done
}
