// Package obex is the object exchange protocol, as much of it as fetching cover art needs.
//
// AVRCP carries a handle for the artwork and nothing else: the image itself comes over a channel of
// its own, running OBEX, with the Basic Imaging profile's names on top. So a controller that wants
// the picture on the screen speaks a third protocol to a third channel, and this is it.
//
// Only the client half, and only the parts a pull needs — connect, get, disconnect. Nothing here
// serves anything, because nothing asks this device for an image.
package obex

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ErrShort is a buffer that does not hold what its header says.
var ErrShort = errors.New("obex: shorter than its header says")

// What a request asks for. The high bit says it is the last packet of its kind, which for anything
// that fits in one packet is always.
const (
	OpConnect    = 0x80
	OpDisconnect = 0x81
	OpPut        = 0x02
	OpGet        = 0x03
	OpAbort      = 0xff

	// Final is set on the last packet of a request. A get that is fetching the rest of a body
	// sends it on every packet, because each one is a whole request in itself.
	Final = 0x80
)

// How a request was answered. These are the http codes shifted into a byte, which is where OBEX
// took them from.
const (
	Continue    = 0x90 // more to come, ask again
	OK          = 0xa0 // done, and this is the last of it
	BadRequest  = 0xc0
	NotFound    = 0xc4
	Unsupported = 0xc3
)

// Header identifiers. The top two bits say how the value is encoded, which is what lets a reader
// skip a header it does not know.
const (
	HeaderCount       = 0xc0
	HeaderName        = 0x01
	HeaderType        = 0x42
	HeaderLength      = 0xc3
	HeaderBody        = 0x48
	HeaderEndOfBody   = 0x49
	HeaderWho         = 0x4a
	HeaderConnection  = 0xcb
	HeaderTarget      = 0x46
	HeaderDescription = 0x05
)

// How a header carries its value, from the top two bits of its identifier.
const (
	encodingUnicode = 0x00
	encodingBytes   = 0x40
	encodingByte    = 0x80
	encodingUint32  = 0xc0
)

// Header is one field of a packet.
//
// The value is always kept as bytes. A unicode header is UTF-16 big endian with a trailing pair of
// zeros on the wire, and Text puts one in that form.
type Header struct {
	ID    byte
	Value []byte
}

// Text is a unicode header, encoded the way the wire wants it.
//
// UTF-16 big endian and null terminated. An image handle is seven ascii digits, which is fourteen
// bytes here plus the two for the terminator.
func Text(id byte, s string) Header {
	out := make([]byte, 0, len(s)*2+2)
	for _, r := range s {
		// Anything outside the basic plane would need a surrogate pair. Nothing this sends has any:
		// handles are digits and types are ascii.
		if r > 0xffff {
			r = 0xfffd
		}
		out = binary.BigEndian.AppendUint16(out, uint16(r))
	}
	return Header{ID: id, Value: append(out, 0, 0)}
}

// Bytes is a byte sequence header.
func Bytes(id byte, v []byte) Header { return Header{ID: id, Value: v} }

// Uint32 is a four byte header, which is how a connection identifier travels.
func Uint32(id byte, v uint32) Header {
	return Header{ID: id, Value: binary.BigEndian.AppendUint32(nil, v)}
}

// Uint32 reads a four byte header's value.
func (h Header) Uint32() (uint32, bool) {
	if h.ID&0xc0 != encodingUint32 || len(h.Value) != 4 {
		return 0, false
	}
	return binary.BigEndian.Uint32(h.Value), true
}

// Marshal writes one header.
func (h Header) Marshal() []byte {
	switch h.ID & 0xc0 {
	case encodingByte:
		v := byte(0)
		if len(h.Value) > 0 {
			v = h.Value[0]
		}
		return []byte{h.ID, v}

	case encodingUint32:
		out := make([]byte, 5)
		out[0] = h.ID
		copy(out[1:], h.Value)
		return out
	}

	// Unicode and byte sequences both carry a length, and it counts the identifier and the length
	// itself as well as the value.
	out := make([]byte, 3, 3+len(h.Value))
	out[0] = h.ID
	binary.BigEndian.PutUint16(out[1:], uint16(3+len(h.Value)))
	return append(out, h.Value...)
}

// ParseHeaders reads a run of them.
func ParseHeaders(buf []byte) ([]Header, error) {
	var out []Header

	for len(buf) > 0 {
		id := buf[0]

		switch id & 0xc0 {
		case encodingByte:
			if len(buf) < 2 {
				return out, ErrShort
			}
			out = append(out, Header{ID: id, Value: []byte{buf[1]}})
			buf = buf[2:]

		case encodingUint32:
			if len(buf) < 5 {
				return out, ErrShort
			}
			out = append(out, Header{ID: id, Value: buf[1:5]})
			buf = buf[5:]

		case encodingUnicode, encodingBytes:
			if len(buf) < 3 {
				return out, ErrShort
			}

			// The length counts the identifier and itself as well as the value, so one under three
			// describes a header smaller than its own header and would leave a reader where it
			// started.
			n := int(binary.BigEndian.Uint16(buf[1:]))
			if n < 3 || len(buf) < n {
				return out, fmt.Errorf("obex: a header says %d bytes and %d are left", n, len(buf))
			}
			out = append(out, Header{ID: id, Value: buf[3:n]})
			buf = buf[n:]

		default:
			return out, fmt.Errorf("obex: a header encoded %#02x, which is not one of the four",
				id&0xc0)
		}
	}
	return out, nil
}

// Find is the first header with this identifier.
func Find(headers []Header, id byte) (Header, bool) {
	for _, h := range headers {
		if h.ID == id {
			return h, true
		}
	}
	return Header{}, false
}
