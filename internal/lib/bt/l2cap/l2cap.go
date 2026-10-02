// Package l2cap speaks L2CAP, the layer everything else in Bluetooth Classic is carried on.
//
// Above HCI and below the profiles: a phone that wants to play music opens an SDP channel to find
// out what this device is, then an AVDTP one to negotiate and stream. Both are L2CAP channels, and
// this is what opens them.
//
// Bytes in, bytes out. Nothing here touches the radio — it is handed whole PDUs by whatever
// reassembled them and hands back what to send, which is what makes a stack with no hardware on
// this side of it testable at all.
//
// Basic mode only. Retransmission and streaming modes exist and A2DP does not need them: the
// profile runs over basic mode and the codec tolerates loss better than a retransmit would help.
package l2cap

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// The channels that are always there. Everything else is opened and given a number at the time.
const (
	// CIDSignalling carries the commands that open and close the rest.
	CIDSignalling = 0x0001

	// CIDConnectionless is broadcast data, which nothing here sends or wants.
	CIDConnectionless = 0x0002

	// CIDDynamic is the first number a channel of our own may be given. Below it is reserved.
	CIDDynamic = 0x0040
)

// The protocol and service multiplexers worth naming: what a far end asks for when it opens a
// channel, and how it says what the channel is for.
const (
	PSMSDP         = 0x0001
	PSMAVCTP       = 0x0017 // remote control, which is AVRCP
	PSMAVDTP       = 0x0019 // the audio stream itself
	PSMAVCTPBrowse = 0x001b // remote control again, for the half about which player
)

// Header is the length and channel every frame starts with.
const Header = 4

// MTUDefault is what a channel carries before anyone says otherwise, and MTUSignalling is the
// smaller one the signalling channel is guaranteed.
const (
	MTUDefault    = 672
	MTUSignalling = 48
)

// ErrShort is a buffer that does not hold what its header says. It is the ordinary state of a
// stream being read, not a fault, so callers tell it apart from a malformed packet.
var ErrShort = errors.New("l2cap: shorter than its header says")

// Frame is one L2CAP packet: a channel and what was sent on it.
type Frame struct {
	CID     uint16
	Payload []byte
}

// ParseFrame reads a frame. The payload points into buf rather than being copied, because the
// caller already owns a reassembled PDU and copying it again helps nobody.
func ParseFrame(buf []byte) (Frame, error) {
	if len(buf) < Header {
		return Frame{}, ErrShort
	}

	n := int(binary.LittleEndian.Uint16(buf))
	if len(buf) != Header+n {
		return Frame{}, fmt.Errorf("l2cap: %d bytes for a frame saying %d", len(buf)-Header, n)
	}

	return Frame{CID: binary.LittleEndian.Uint16(buf[2:]), Payload: buf[Header:]}, nil
}

// Marshal writes a frame for sending.
func (f Frame) Marshal() []byte {
	out := make([]byte, Header+len(f.Payload))
	binary.LittleEndian.PutUint16(out, uint16(len(f.Payload)))
	binary.LittleEndian.PutUint16(out[2:], f.CID)
	copy(out[Header:], f.Payload)
	return out
}

// What a signalling command is asking for.
const (
	CodeReject             = 0x01
	CodeConnectRequest     = 0x02
	CodeConnectResponse    = 0x03
	CodeConfigRequest      = 0x04
	CodeConfigResponse     = 0x05
	CodeDisconnectRequest  = 0x06
	CodeDisconnectResponse = 0x07
	CodeEchoRequest        = 0x08
	CodeEchoResponse       = 0x09
	CodeInfoRequest        = 0x0a
	CodeInfoResponse       = 0x0b
)

// commandHeader is the code, the identifier and the length.
const commandHeader = 4

// Command is one signalling command, before anyone has worked out which one.
//
// The identifier is what pairs a response with its request. A far end may have several outstanding
// and answer them in any order, so matching on the code alone would hand the wrong answer to the
// wrong question.
type Command struct {
	Code byte
	ID   byte
	Data []byte
}

// ParseCommands reads every command in one signalling frame.
//
// Several may share a frame, which is why this is a list. A trailing byte that is not a whole
// command is a malformed frame rather than a short read: the frame is already complete by the time
// it gets here.
func ParseCommands(payload []byte) ([]Command, error) {
	var out []Command

	for len(payload) > 0 {
		if len(payload) < commandHeader {
			return nil, fmt.Errorf("l2cap: %d bytes left over, less than a command header",
				len(payload))
		}

		n := int(binary.LittleEndian.Uint16(payload[2:]))
		if len(payload) < commandHeader+n {
			return nil, fmt.Errorf("l2cap: a command says %d bytes and %d are left",
				n, len(payload)-commandHeader)
		}

		out = append(out, Command{
			Code: payload[0],
			ID:   payload[1],
			Data: payload[commandHeader : commandHeader+n],
		})
		payload = payload[commandHeader+n:]
	}

	if len(out) == 0 {
		return nil, errors.New("l2cap: a signalling frame with no commands in it")
	}
	return out, nil
}

// Marshal writes one command.
func (c Command) Marshal() []byte {
	out := make([]byte, commandHeader+len(c.Data))
	out[0] = c.Code
	out[1] = c.ID
	binary.LittleEndian.PutUint16(out[2:], uint16(len(c.Data)))
	copy(out[commandHeader:], c.Data)
	return out
}

// Signal wraps commands into a frame on the signalling channel.
func Signal(commands ...Command) Frame {
	var payload []byte
	for _, c := range commands {
		payload = append(payload, c.Marshal()...)
	}
	return Frame{CID: CIDSignalling, Payload: payload}
}

// Connect is a far end asking to open a channel: what it wants to talk to, and the number it will
// know the channel by at its own end.
type Connect struct {
	PSM       uint16
	SourceCID uint16
}

// Command writes a connection request.
func (r Connect) Command(id byte) Command {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint16(data, r.PSM)
	binary.LittleEndian.PutUint16(data[2:], r.SourceCID)

	return Command{Code: CodeConnectRequest, ID: id, Data: data}
}

func ParseConnect(c Command) (Connect, error) {
	if len(c.Data) < 4 {
		return Connect{}, ErrShort
	}
	return Connect{
		PSM:       binary.LittleEndian.Uint16(c.Data),
		SourceCID: binary.LittleEndian.Uint16(c.Data[2:]),
	}, nil
}

// How a connection request was answered.
const (
	ConnectSuccess     = 0x0000
	ConnectPending     = 0x0001
	ConnectBadPSM      = 0x0002
	ConnectSecurity    = 0x0003
	ConnectNoResources = 0x0004
)

// Connected answers a connection request. The two channel numbers are each end's own: a channel has
// a different number at either side and neither may assume the other's.
type Connected struct {
	DestinationCID uint16
	SourceCID      uint16
	Result         uint16
	Status         uint16
}

func (r Connected) Command(id byte) Command {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint16(data, r.DestinationCID)
	binary.LittleEndian.PutUint16(data[2:], r.SourceCID)
	binary.LittleEndian.PutUint16(data[4:], r.Result)
	binary.LittleEndian.PutUint16(data[6:], r.Status)

	return Command{Code: CodeConnectResponse, ID: id, Data: data}
}

func ParseConnected(c Command) (Connected, error) {
	if len(c.Data) < 8 {
		return Connected{}, ErrShort
	}
	return Connected{
		DestinationCID: binary.LittleEndian.Uint16(c.Data),
		SourceCID:      binary.LittleEndian.Uint16(c.Data[2:]),
		Result:         binary.LittleEndian.Uint16(c.Data[4:]),
		Status:         binary.LittleEndian.Uint16(c.Data[6:]),
	}, nil
}

// Disconnect closes a channel, and the response says the same two numbers back.
type Disconnect struct {
	DestinationCID uint16
	SourceCID      uint16
}

func ParseDisconnect(c Command) (Disconnect, error) {
	if len(c.Data) < 4 {
		return Disconnect{}, ErrShort
	}
	return Disconnect{
		DestinationCID: binary.LittleEndian.Uint16(c.Data),
		SourceCID:      binary.LittleEndian.Uint16(c.Data[2:]),
	}, nil
}

func (d Disconnect) Command(code, id byte) Command {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint16(data, d.DestinationCID)
	binary.LittleEndian.PutUint16(data[2:], d.SourceCID)

	return Command{Code: code, ID: id, Data: data}
}

// Why a command could not be understood at all, which is different from a request being refused.
const (
	RejectNotUnderstood = 0x0000
	RejectBadMTU        = 0x0001
	RejectBadCID        = 0x0002
)

// Reject says a command made no sense. Sent rather than ignoring one, because a far end waiting on
// an answer it will never get is worse than being told no.
func Reject(id byte, reason uint16, extra ...uint16) Command {
	data := make([]byte, 2+2*len(extra))
	binary.LittleEndian.PutUint16(data, reason)

	for i, v := range extra {
		binary.LittleEndian.PutUint16(data[2+2*i:], v)
	}
	return Command{Code: CodeReject, ID: id, Data: data}
}
