package cast

import (
	"encoding/binary"
	"fmt"
)

// Framing. Each message on the connection is a four byte big-endian length and then that many
// bytes of protobuf.
//
// Big-endian, where everything else in this tree that counts bytes is little. It is the one place
// a habit produces a length of two billion and a reader that waits for the rest of it forever.

// lengthBytes is the prefix.
const lengthBytes = 4

// MaxMessage is the largest message this will take.
//
// The protocol sets no limit and a sender sends nothing near this — a media status with artwork
// urls is a few kilobytes. The cap is here because the length arrives before the bytes do, so a
// wrong or hostile one otherwise asks this process to allocate whatever it says.
const MaxMessage = 1 << 20

// Frame writes a message with its length in front.
func Frame(m Message) []byte {
	body := m.Marshal()

	out := make([]byte, lengthBytes+len(body))
	binary.BigEndian.PutUint32(out, uint32(len(body)))
	copy(out[lengthBytes:], body)

	return out
}

// Next reads one message off the front of a buffer and says how much of it that took.
//
// A short buffer is reported as short rather than as broken: TCP hands over whatever has arrived,
// and half a message is the ordinary state of a stream being read.
func Next(buf []byte) (Message, int, error) {
	if len(buf) < lengthBytes {
		return Message{}, 0, ErrShort
	}

	n := int(binary.BigEndian.Uint32(buf))
	if n > MaxMessage {
		return Message{}, 0, fmt.Errorf("cast: a message claiming %d bytes, more than the %d allowed",
			n, MaxMessage)
	}
	if len(buf) < lengthBytes+n {
		return Message{}, 0, ErrShort
	}

	m, err := Unmarshal(buf[lengthBytes : lengthBytes+n])
	if err != nil {
		return Message{}, 0, err
	}
	return m, lengthBytes + n, nil
}
