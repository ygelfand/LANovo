// Package cast speaks CASTV2, the protocol a Chromecast sender talks to a receiver.
//
// This device is the receiver. The Android one is gone — keeping it meant keeping zygote and
// system_server, which is what unloads the wifi driver and fights us for the hardware — so the
// protocol is spoken here instead.
//
// # THE SHAPE OF IT
//
// A sender opens TLS to port 8009 and sends length-prefixed protobuf messages. Each carries a
// source, a destination, a namespace, and a payload that is almost always JSON. The namespace is
// what says which conversation a message belongs to, and there are four that matter:
//
//	urn:x-cast:com.google.cast.tp.connection    opening and closing a conversation
//	urn:x-cast:com.google.cast.tp.heartbeat     PING and PONG, and the timeout that drops a sender
//	urn:x-cast:com.google.cast.receiver         what is running, and launching or stopping it
//	urn:x-cast:com.google.cast.media            what is playing, and the transport controls
//
// Everything is bytes in and bytes out. The TLS and the network live above this; what is here can
// be driven by a test.
//
// # WHY THE MESSAGE IS ENCODED BY HAND
//
// CastMessage is seven fields and has not changed since 2013, and this is written against
// protowire — the protobuf module's own low-level encoder, which is already a dependency — rather
// than generated from cast_channel.proto. That keeps protoc out of the build for a message small
// enough to check against golden bytes, and the varint and tag handling is still the library's
// rather than ours.
package cast

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

// ErrShort is a buffer that does not hold what its header says.
var ErrShort = errors.New("cast: shorter than its header says")

// The field numbers of CastMessage, from cast_channel.proto.
const (
	fieldProtocol      = 1
	fieldSource        = 2
	fieldDestination   = 3
	fieldNamespace     = 4
	fieldPayloadType   = 5
	fieldPayloadUTF8   = 6
	fieldPayloadBinary = 7
)

// The only protocol version there has been.
const ProtocolV2 = 0

// What the payload is. Almost everything is JSON in the string; binary is for the few namespaces
// that carry their own protobuf, and this device implements none of them.
const (
	PayloadString = 0
	PayloadBinary = 1
)

// Message is one CASTV2 message.
type Message struct {
	// Source and Destination are the endpoints within the connection, not network addresses. A
	// sender is "sender-0" and the receiver itself is "receiver-0"; an application gets its own
	// transport id when it launches.
	Source      string
	Destination string

	Namespace string

	// Payload is the JSON. Binary is set instead for the namespaces that carry protobuf, and both
	// are never set at once.
	Payload string
	Binary  []byte
}

// Broadcast is the destination that means every endpoint, which is what a receiver sends status to
// when it does not know who is listening.
const Broadcast = "*"

// The endpoint ids that always exist. An application launched later gets one of its own.
const (
	SenderID   = "sender-0"
	ReceiverID = "receiver-0"
)

// Marshal writes the message as protobuf.
func (m Message) Marshal() []byte {
	var out []byte

	out = protowire.AppendTag(out, fieldProtocol, protowire.VarintType)
	out = protowire.AppendVarint(out, ProtocolV2)

	out = protowire.AppendTag(out, fieldSource, protowire.BytesType)
	out = protowire.AppendString(out, m.Source)

	out = protowire.AppendTag(out, fieldDestination, protowire.BytesType)
	out = protowire.AppendString(out, m.Destination)

	out = protowire.AppendTag(out, fieldNamespace, protowire.BytesType)
	out = protowire.AppendString(out, m.Namespace)

	kind := PayloadString
	if m.Binary != nil {
		kind = PayloadBinary
	}
	out = protowire.AppendTag(out, fieldPayloadType, protowire.VarintType)
	out = protowire.AppendVarint(out, uint64(kind))

	if m.Binary != nil {
		out = protowire.AppendTag(out, fieldPayloadBinary, protowire.BytesType)
		out = protowire.AppendBytes(out, m.Binary)
	} else {
		out = protowire.AppendTag(out, fieldPayloadUTF8, protowire.BytesType)
		out = protowire.AppendString(out, m.Payload)
	}

	return out
}

// Unmarshal reads one.
//
// Unknown fields are skipped rather than refused: the schema has grown before and a receiver that
// rejects a message for carrying something it does not know about is a receiver that stops working
// when the sender is updated.
func Unmarshal(buf []byte) (Message, error) {
	var m Message

	for len(buf) > 0 {
		number, kind, n := protowire.ConsumeTag(buf)
		if n < 0 {
			return Message{}, fmt.Errorf("cast: %w", protowire.ParseError(n))
		}
		buf = buf[n:]

		switch {
		case number == fieldSource && kind == protowire.BytesType:
			m.Source, n = protowire.ConsumeString(buf)
		case number == fieldDestination && kind == protowire.BytesType:
			m.Destination, n = protowire.ConsumeString(buf)
		case number == fieldNamespace && kind == protowire.BytesType:
			m.Namespace, n = protowire.ConsumeString(buf)
		case number == fieldPayloadUTF8 && kind == protowire.BytesType:
			m.Payload, n = protowire.ConsumeString(buf)

		case number == fieldPayloadBinary && kind == protowire.BytesType:
			var b []byte
			b, n = protowire.ConsumeBytes(buf)
			if n >= 0 {
				// Copied, because the buffer it came out of is the read buffer and is about to
				// hold the next message.
				m.Binary = append([]byte(nil), b...)
			}

		default:
			n = protowire.ConsumeFieldValue(number, kind, buf)
		}

		if n < 0 {
			return Message{}, fmt.Errorf("cast: field %d: %w", number, protowire.ParseError(n))
		}
		buf = buf[n:]
	}

	return m, nil
}
