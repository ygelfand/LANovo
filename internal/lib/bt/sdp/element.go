// Package sdp speaks Bluetooth's Service Discovery Protocol, which is how a phone finds out what
// this device is before it tries to play anything through it.
//
// Not Session Description Protocol, the "v=0 / m=audio" text blob that WebRTC and SIP use and that
// pion/sdp implements. Same three letters, unrelated protocols — which is why this sits under
// internal/lib/bt with the rest of the Bluetooth stack rather than at the top of internal/lib.
//
// A2DP needs very little of it: one service record saying "there is an audio sink here, it speaks
// AVDTP, and this is the channel to reach it on". A phone browses, reads that, and then opens the
// channel the record named. Getting it wrong is not a stream that sounds bad, it is a device that
// does not appear.
//
// Bytes in, bytes out. Everything is built on one encoding — the data element — which is a tagged
// value that nests, and which the whole protocol is made of.
package sdp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ErrShort is a buffer that does not hold what its header says.
var ErrShort = errors.New("sdp: shorter than its header says")

// What a data element is. The type lives in the top five bits of the first byte.
const (
	TypeNil         = 0
	TypeUint        = 1
	TypeInt         = 2
	TypeUUID        = 3
	TypeText        = 4
	TypeBool        = 5
	TypeSequence    = 6
	TypeAlternative = 7
	TypeURL         = 8
)

// The bottom three bits are a size index rather than a size.
//
// For the fixed width types it names a width: one, two, four, eight or sixteen bytes. For the ones
// that vary it says how many bytes of length follow. The two meanings share three bits, which is
// the part of this encoding that catches people.
const (
	size1   = 0
	size2   = 1
	size4   = 2
	size8   = 3
	size16  = 4
	size8b  = 5 // one byte of length follows
	size16b = 6 // two bytes
	size32b = 7 // four bytes
)

// Element is one value: a number, a UUID, a string, or a sequence of them.
//
// Held as the bytes it was written as rather than as a Go value, because a record is built once and
// then sent unchanged, and because a UUID is sixteen bytes or two depending on how it was written
// and both mean the same thing.
type Element struct {
	Type byte

	// Value is the payload, without the header.
	Value []byte

	// Children are what a sequence or an alternative holds. Value is empty for those.
	Children []Element
}

// Uint is an unsigned integer in the smallest width that holds it.
//
// Smallest rather than always four: a record is read by a phone with a small buffer and the spec
// writes numbers tightly, so a wide one where a narrow one would do is unusual enough to be worth
// not doing.
func Uint(v uint32) Element {
	switch {
	case v <= 0xff:
		return Element{Type: TypeUint, Value: []byte{byte(v)}}
	case v <= 0xffff:
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(v))
		return Element{Type: TypeUint, Value: b}
	}

	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return Element{Type: TypeUint, Value: b}
}

// Uint16 and Uint32 are numbers at a width chosen rather than fitted.
//
// Uint writes the smallest width that holds a value, which is right for an ordinary number and
// wrong wherever the width itself means something. An attribute list is the case: two bytes there
// is one identifier and four is a range, so a range whose ends both fit in two bytes has to be
// written wide anyway or it silently becomes a single attribute.
func Uint16(v uint16) Element {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	return Element{Type: TypeUint, Value: b}
}

func Uint32(v uint32) Element {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return Element{Type: TypeUint, Value: b}
}

// UUID16 is a short UUID, which is how every assigned Bluetooth identifier is written.
func UUID16(v uint16) Element {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	return Element{Type: TypeUUID, Value: b}
}

// UUID32 is the four byte form, for an identifier that does not fit the short one.
func UUID32(v uint32) Element {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return Element{Type: TypeUUID, Value: b}
}

// Text is a string.
func Text(s string) Element { return Element{Type: TypeText, Value: []byte(s)} }

// Bool is true or false.
func Bool(b bool) Element {
	v := byte(0)
	if b {
		v = 1
	}
	return Element{Type: TypeBool, Value: []byte{v}}
}

// Sequence is an ordered list, which most of a service record is made of.
func Sequence(children ...Element) Element {
	return Element{Type: TypeSequence, Children: children}
}

// Alternative is a list of which any one will do.
func Alternative(children ...Element) Element {
	return Element{Type: TypeAlternative, Children: children}
}

// Nil is the absent value.
func Nil() Element { return Element{Type: TypeNil} }

// nests reports whether this type holds other elements rather than bytes.
func nests(t byte) bool { return t == TypeSequence || t == TypeAlternative }

// Marshal writes the element and everything under it.
func (e Element) Marshal() ([]byte, error) {
	var body []byte

	if nests(e.Type) {
		for _, c := range e.Children {
			b, err := c.Marshal()
			if err != nil {
				return nil, err
			}
			body = append(body, b...)
		}
	} else {
		body = e.Value
	}

	head, err := header(e.Type, len(body))
	if err != nil {
		return nil, err
	}
	return append(head, body...), nil
}

// header is the tag byte and, for the types that need one, the length after it.
func header(t byte, n int) ([]byte, error) {
	if t > TypeURL {
		return nil, fmt.Errorf("sdp: %d is not a data element type", t)
	}

	tag := func(index byte) byte { return t<<3 | index }

	// Nil carries nothing, and says so with the width index that would otherwise mean one byte.
	if t == TypeNil {
		return []byte{tag(size1)}, nil
	}

	// The fixed width types name their width in the index and carry no length.
	if !nests(t) && t != TypeText && t != TypeURL {
		switch n {
		case 1:
			return []byte{tag(size1)}, nil
		case 2:
			return []byte{tag(size2)}, nil
		case 4:
			return []byte{tag(size4)}, nil
		case 8:
			return []byte{tag(size8)}, nil
		case 16:
			return []byte{tag(size16)}, nil
		}
		return nil, fmt.Errorf("sdp: a type %d value cannot be %d bytes", t, n)
	}

	// The rest carry a length, in as few bytes as hold it.
	switch {
	case n <= 0xff:
		return []byte{tag(size8b), byte(n)}, nil
	case n <= 0xffff:
		out := []byte{tag(size16b), 0, 0}
		binary.BigEndian.PutUint16(out[1:], uint16(n))
		return out, nil
	}

	out := []byte{tag(size32b), 0, 0, 0, 0}
	binary.BigEndian.PutUint32(out[1:], uint32(n))
	return out, nil
}

// ParseElement reads one element off the front of buf and says how much of buf it took.
func ParseElement(buf []byte) (Element, int, error) {
	if len(buf) < 1 {
		return Element{}, 0, ErrShort
	}

	t := buf[0] >> 3
	index := buf[0] & 0x7

	if t > TypeURL {
		return Element{}, 0, fmt.Errorf("sdp: %d is not a data element type", t)
	}

	n, head, err := length(t, index, buf)
	if err != nil {
		return Element{}, 0, err
	}
	if len(buf) < head+n {
		return Element{}, 0, ErrShort
	}

	body := buf[head : head+n]

	if !nests(t) {
		return Element{Type: t, Value: body}, head + n, nil
	}

	// A sequence is read by reading what is inside it until there is none left. A child that runs
	// past the end is a malformed record rather than more to read: the length already said where
	// the sequence stops.
	e := Element{Type: t}
	for len(body) > 0 {
		child, used, err := ParseElement(body)
		if err != nil {
			return Element{}, 0, fmt.Errorf("sdp: inside a sequence: %w", err)
		}
		e.Children = append(e.Children, child)
		body = body[used:]
	}
	return e, head + n, nil
}

// length works out how long an element's body is and how many bytes said so.
func length(t, index byte, buf []byte) (n, head int, err error) {
	if t == TypeNil {
		return 0, 1, nil
	}

	switch index {
	case size1:
		return 1, 1, nil
	case size2:
		return 2, 1, nil
	case size4:
		return 4, 1, nil
	case size8:
		return 8, 1, nil
	case size16:
		return 16, 1, nil

	case size8b:
		if len(buf) < 2 {
			return 0, 0, ErrShort
		}
		return int(buf[1]), 2, nil

	case size16b:
		if len(buf) < 3 {
			return 0, 0, ErrShort
		}
		return int(binary.BigEndian.Uint16(buf[1:])), 3, nil

	case size32b:
		if len(buf) < 5 {
			return 0, 0, ErrShort
		}
		return int(binary.BigEndian.Uint32(buf[1:])), 5, nil
	}
	return 0, 0, fmt.Errorf("sdp: size index %d", index)
}

// Uint reads an element's value as a number, for the widths a number comes in.
func (e Element) Uint() (uint32, bool) {
	if e.Type != TypeUint && e.Type != TypeUUID {
		return 0, false
	}

	switch len(e.Value) {
	case 1:
		return uint32(e.Value[0]), true
	case 2:
		return uint32(binary.BigEndian.Uint16(e.Value)), true
	case 4:
		return binary.BigEndian.Uint32(e.Value), true
	}
	return 0, false
}
