package l2cap

import (
	"encoding/binary"
	"fmt"
)

// Configuration is the round of options two ends trade after a channel opens and before anything is
// sent on it. For A2DP the one that matters is the MTU: a phone announces how large a packet it
// will send, and an audio packet larger than what was agreed is a stream that stops.

// The options worth knowing. Anything else is either refused or ignored depending on its hint bit.
const (
	OptionMTU     = 0x01
	OptionFlush   = 0x02
	OptionQoS     = 0x03
	OptionRetrans = 0x04
)

// OptionHint is set on an option the sender is only suggesting. One with it set may be skipped
// where one without it has to be answered — refusing a hint is how a configuration round never
// finishes.
const OptionHint = 0x80

// Option is one configuration option, with the hint bit already taken off the type.
type Option struct {
	Type  byte
	Hint  bool
	Value []byte
}

// MTU reads an MTU option's value.
func (o Option) MTU() (uint16, bool) {
	if o.Type != OptionMTU || len(o.Value) < 2 {
		return 0, false
	}
	return binary.LittleEndian.Uint16(o.Value), true
}

// Marshal writes one option.
func (o Option) Marshal() []byte {
	t := o.Type
	if o.Hint {
		t |= OptionHint
	}

	out := make([]byte, 2+len(o.Value))
	out[0] = t
	out[1] = byte(len(o.Value))
	copy(out[2:], o.Value)
	return out
}

// MTUOption is the one option this stack sends.
func MTUOption(mtu uint16) Option {
	v := make([]byte, 2)
	binary.LittleEndian.PutUint16(v, mtu)

	return Option{Type: OptionMTU, Value: v}
}

// ParseOptions reads a run of options.
func ParseOptions(buf []byte) ([]Option, error) {
	var out []Option

	for len(buf) > 0 {
		if len(buf) < 2 {
			return nil, fmt.Errorf("l2cap: %d bytes left over, less than an option header", len(buf))
		}

		n := int(buf[1])
		if len(buf) < 2+n {
			return nil, fmt.Errorf("l2cap: an option says %d bytes and %d are left", n, len(buf)-2)
		}

		out = append(out, Option{
			Type:  buf[0] &^ OptionHint,
			Hint:  buf[0]&OptionHint != 0,
			Value: buf[2 : 2+n],
		})
		buf = buf[2+n:]
	}
	return out, nil
}

// ConfigFlagContinues says more options are coming in another command, because they did not fit in
// one. A response to a continued request has to carry the flag back or the far end waits forever.
const ConfigFlagContinues = 0x0001

// Configure is a far end saying how it wants a channel to behave.
type Configure struct {
	DestinationCID uint16
	Flags          uint16
	Options        []Option
}

// Continues reports whether more options are coming.
func (c Configure) Continues() bool { return c.Flags&ConfigFlagContinues != 0 }

// MTU is the largest packet the far end says it will send, and whether it said.
//
// Absent means the default rather than nothing: a channel with no MTU option agreed carries 672
// bytes, and treating silence as zero would close a channel that is working.
func (c Configure) MTU() uint16 {
	for _, o := range c.Options {
		if mtu, ok := o.MTU(); ok {
			return mtu
		}
	}
	return MTUDefault
}

// Unknown is the options that have to be answered and cannot be, which is what a configuration
// response full of unknown options carries back.
//
// Hints are left out on purpose: the whole point of the bit is that skipping one is allowed.
func (c Configure) Unknown() []byte {
	var out []byte
	for _, o := range c.Options {
		switch {
		case o.Hint:
		case o.Type == OptionMTU, o.Type == OptionFlush, o.Type == OptionQoS:

		// Retransmission is known, and answered with a counter-proposal rather than with
		// puzzlement. Calling an option we understand unknown tells a far end that needs it there
		// is nothing to negotiate.
		case o.Type == OptionRetrans:

		default:
			out = append(out, o.Type)
		}
	}
	return out
}

// How a channel carries what is sent on it.
const (
	ModeBasic   = 0x00
	ModeRetrans = 0x01
	ModeFlow    = 0x02
	ModeERTM    = 0x03
	ModeStream  = 0x04
)

// retransFields is the option's width: a mode, a window, a transmit count, two timers and a size.
const retransFields = 9

// Mode is the one a configuration asks for, and whether it asked at all. Silence means basic, which
// is what a channel is until something says otherwise.
func (c Configure) Mode() (mode byte, asked bool) {
	for _, o := range c.Options {
		if o.Type == OptionRetrans && len(o.Value) >= 1 {
			return o.Value[0], true
		}
	}
	return ModeBasic, false
}

// BasicMode is the counter-proposal to a mode this stack does not speak.
//
// The other fields are a window, a transmit count, two timers and a packet size, and none of them
// mean anything in basic mode — but the option is a fixed width, so they are sent as zeros rather
// than left out.
func BasicMode() Option {
	return Option{Type: OptionRetrans, Value: make([]byte, retransFields)}
}

func ParseConfigure(c Command) (Configure, error) {
	if len(c.Data) < 4 {
		return Configure{}, ErrShort
	}

	options, err := ParseOptions(c.Data[4:])
	if err != nil {
		return Configure{}, err
	}

	return Configure{
		DestinationCID: binary.LittleEndian.Uint16(c.Data),
		Flags:          binary.LittleEndian.Uint16(c.Data[2:]),
		Options:        options,
	}, nil
}

func (c Configure) Command(id byte) Command {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint16(data, c.DestinationCID)
	binary.LittleEndian.PutUint16(data[2:], c.Flags)

	for _, o := range c.Options {
		data = append(data, o.Marshal()...)
	}
	return Command{Code: CodeConfigRequest, ID: id, Data: data}
}

// How a configuration round was answered.
const (
	ConfigSuccess      = 0x0000
	ConfigUnacceptable = 0x0001
	ConfigRejected     = 0x0002
	ConfigUnknown      = 0x0003
	ConfigPending      = 0x0004
)

// Configured answers a configuration request.
type Configured struct {
	SourceCID uint16
	Flags     uint16
	Result    uint16
	Options   []Option
}

func (r Configured) Command(id byte) Command {
	data := make([]byte, 6)
	binary.LittleEndian.PutUint16(data, r.SourceCID)
	binary.LittleEndian.PutUint16(data[2:], r.Flags)
	binary.LittleEndian.PutUint16(data[4:], r.Result)

	for _, o := range r.Options {
		data = append(data, o.Marshal()...)
	}
	return Command{Code: CodeConfigResponse, ID: id, Data: data}
}

func ParseConfigured(c Command) (Configured, error) {
	if len(c.Data) < 6 {
		return Configured{}, ErrShort
	}

	options, err := ParseOptions(c.Data[6:])
	if err != nil {
		return Configured{}, err
	}

	return Configured{
		SourceCID: binary.LittleEndian.Uint16(c.Data),
		Flags:     binary.LittleEndian.Uint16(c.Data[2:]),
		Result:    binary.LittleEndian.Uint16(c.Data[4:]),
		Options:   options,
	}, nil
}
