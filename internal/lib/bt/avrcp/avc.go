package avrcp

import "fmt"

// AV/C frames. A command type, a subunit to address, an opcode, and operands.

// What a command is asking for.
const (
	Control        = 0x0
	Status         = 0x1
	Notify         = 0x3
	GeneralInquiry = 0x4
)

// What a response says. These share the field with the command types, which is why a frame has to
// be read knowing which direction it came from.
const (
	NotImplemented = 0x8
	Accepted       = 0x9
	Rejected       = 0xa
	InTransition   = 0xb
	Stable         = 0xc
	Changed        = 0xd
	Interim        = 0xf
)

// The subunits that get addressed. Panel is the one with the buttons on it; Unit is the device
// itself, which is what identity questions go to.
const (
	SubunitPanel = 0x09
	SubunitUnit  = 0x1f
)

// The opcodes this uses. VendorDependent is the interesting one — everything AVRCP added on top of
// AV/C travels inside it.
const (
	OpVendorDependent = 0x00
	OpUnitInfo        = 0x30
	OpSubunitInfo     = 0x31
	OpPassThrough     = 0x7c
)

// SIG is the Bluetooth SIG's company identifier, which is what marks a vendor-dependent frame as
// carrying an AVRCP pdu rather than some manufacturer's own extension.
const SIG = 0x001958

// avcHeader is the command type, the subunit, and the opcode.
const avcHeader = 3

// AVC is one frame.
type AVC struct {
	// Code is the command type on the way out and the response on the way back. The same four bits
	// either way, meaning different things, so a frame is read knowing which direction it came
	// from.
	Code byte

	Subunit   byte
	SubunitID byte
	Opcode    byte
	Operands  []byte
}

// ParseAVC reads one frame.
func ParseAVC(buf []byte) (AVC, error) {
	if len(buf) < avcHeader {
		return AVC{}, ErrShort
	}

	return AVC{
		Code:      buf[0] & 0x0f,
		Subunit:   buf[1] >> 3,
		SubunitID: buf[1] & 0x07,
		Opcode:    buf[2],
		Operands:  buf[avcHeader:],
	}, nil
}

// Marshal writes it.
func (a AVC) Marshal() []byte {
	out := make([]byte, avcHeader+len(a.Operands))

	out[0] = a.Code & 0x0f
	out[1] = a.Subunit<<3 | a.SubunitID&0x07
	out[2] = a.Opcode
	copy(out[avcHeader:], a.Operands)

	return out
}

// The buttons, as the AV/C panel subunit numbers them. Only the ones a music player has; the
// original list runs to menu navigation for a set-top box.
const (
	OpSelect      = 0x00
	OpVolumeUp    = 0x41
	OpVolumeDown  = 0x42
	OpMute        = 0x43
	OpPlay        = 0x44
	OpStop        = 0x45
	OpPause       = 0x46
	OpRewind      = 0x48
	OpFastForward = 0x49
	OpNext        = 0x4b
	OpPrevious    = 0x4c
)

// Pressed and Released are the two halves of a button.
//
// Both have to be sent. A press with no release leaves the target holding the button down, which
// on a seek turns a tap on fast-forward into running to the end of the track.
const (
	Pressed  = false
	Released = true
)

// PassThrough is a button, as the frame that carries it.
func PassThrough(op byte, released bool) AVC {
	state := op & 0x7f
	if released {
		state |= 0x80
	}

	// The second operand is the length of the operation's own data, which for every button here is
	// none. It is not optional: a frame without it is refused by most targets.
	return AVC{
		Code:     Control,
		Subunit:  SubunitPanel,
		Opcode:   OpPassThrough,
		Operands: []byte{state, 0x00},
	}
}

// Button reads a passthrough frame back: which button, and whether this is the release.
func Button(a AVC) (op byte, released bool, err error) {
	if a.Opcode != OpPassThrough {
		return 0, false, fmt.Errorf("avrcp: opcode %#02x is not a passthrough", a.Opcode)
	}
	if len(a.Operands) < 1 {
		return 0, false, ErrShort
	}
	return a.Operands[0] & 0x7f, a.Operands[0]&0x80 != 0, nil
}

// ButtonName is what a button is called, for a log somebody has to read later.
func ButtonName(op byte) string {
	switch op {
	case OpSelect:
		return "select"
	case OpVolumeUp:
		return "volume up"
	case OpVolumeDown:
		return "volume down"
	case OpMute:
		return "mute"
	case OpPlay:
		return "play"
	case OpStop:
		return "stop"
	case OpPause:
		return "pause"
	case OpRewind:
		return "rewind"
	case OpFastForward:
		return "fast forward"
	case OpNext:
		return "next"
	case OpPrevious:
		return "previous"
	}
	return fmt.Sprintf("button %#02x", op)
}
