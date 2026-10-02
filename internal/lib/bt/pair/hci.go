// Package pair is bonding: letting a phone connect once and be remembered.
//
// Everything else in internal/lib/bt runs over L2CAP, which is data on an open link. This runs
// underneath that, on HCI, because deciding whether a link may exist is the controller's business
// and not the profile's. Nothing here touches the radio — events come in as bytes and commands go
// out as bytes, the same shape as the layers above, so a device that cannot run a test is not
// needed to test it.
//
// What a phone and a speaker actually do:
//
//	the phone connects        connection request   → accept, staying the peripheral
//	is this one known?        link key request     → the stored key, or no and pair again
//	how can you confirm?      io capability        → not at all, so just works
//	is this the number?       user confirmation    → yes, nobody was asked
//	keep this                 link key notification → saved, or it pairs again every boot
package pair

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

// The events this answers. Everything else on the wire belongs to somebody else.
const (
	EventConnectionComplete    = 0x03
	EventConnectionRequest     = 0x04
	EventDisconnectionComplete = 0x05
	EventAuthComplete          = 0x06
	EventEncryptionChange      = 0x08
	EventPINCodeRequest        = 0x16
	EventLinkKeyRequest        = 0x17
	EventLinkKeyNotification   = 0x18
	EventRemoteName            = 0x07
	EventIOCapabilityRequest   = 0x31
	EventIOCapabilityResponse  = 0x32
	EventUserConfirmation      = 0x33
	EventUserPasskeyRequest    = 0x34
	EventSimplePairingComplete = 0x36
)

// The commands it sends back.
const (
	OpAcceptConnection   = 0x0409
	OpRejectConnection   = 0x040a
	OpLinkKeyReply       = 0x040b
	OpLinkKeyNegative    = 0x040c
	OpPINCodeNegative    = 0x040e
	OpIOCapabilityReply  = 0x042b
	OpConfirmReply       = 0x042c
	OpConfirmNegative    = 0x042d
	OpPasskeyNegative    = 0x042f
	OpIOCapabilityNegate = 0x0434
	OpRemoteName         = 0x0419
)

// What this device can do about confirming a pairing.
//
// NoInputNoOutput is what a speaker declares and what gives Just Works: nobody is asked anything
// and the link has no protection against someone in the middle. That is what every Bluetooth
// speaker does and what a phone expects from one.
//
// DisplayYesNo is the interesting one here, because this device has a ten inch screen. Declaring it
// gets numeric comparison — the same six digits on both ends, confirmed by a tap — and with it the
// authentication that Just Works has none of. Nothing selects it yet; the field is where it would
// go.
const (
	DisplayOnly     = 0x00
	DisplayYesNo    = 0x01
	KeyboardOnly    = 0x02
	NoInputNoOutput = 0x03
)

// What the pairing has to achieve. The bonding part is the one that matters: without it the
// controller generates no link key, nothing is saved, and the phone pairs again every single time
// it connects — which looks to somebody in the room like the speaker forgetting them.
const (
	NoBonding              = 0x00
	NoBondingMITM          = 0x01
	DedicatedBonding       = 0x02
	DedicatedBondingMITM   = 0x03
	GeneralBonding         = 0x04
	GeneralBondingWithMITM = 0x05
)

// Link types a connection request can be for.
const (
	LinkSCO  = 0x00
	LinkACL  = 0x01
	LinkESCO = 0x02
)

// Why a connection was refused. Only the ones there is reason to send.
const (
	RejectLimitedResources = 0x0d
	RejectSecurity         = 0x0e
	RejectUnacceptableAddr = 0x0f
)

// The kinds of link key a notification can carry.
const (
	KeyCombination         = 0x00
	KeyLocalUnit           = 0x01
	KeyRemoteUnit          = 0x02
	KeyDebug               = 0x03
	KeyUnauthenticated     = 0x04
	KeyAuthenticated       = 0x05
	KeyChangedCombination  = 0x06
	KeyUnauthenticatedP256 = 0x07
	KeyAuthenticatedP256   = 0x08
)

// Addr is a Bluetooth device address.
//
// Held in the order it goes on the wire, which is backwards from how anyone writes one down. The
// difference is invisible until a key is looked up under an address stored the other way round and
// a phone that paired yesterday is a stranger today, so the reversal happens in one place: here.
type Addr [6]byte

// ParseAddr reads an address out of the front of a buffer.
func ParseAddr(buf []byte) (Addr, bool) {
	var a Addr
	if len(buf) < len(a) {
		return a, false
	}
	copy(a[:], buf)
	return a, true
}

// String writes it the way it is printed everywhere else, most significant byte first.
func (a Addr) String() string {
	var b strings.Builder
	for i := len(a) - 1; i >= 0; i-- {
		if b.Len() > 0 {
			b.WriteByte(':')
		}
		fmt.Fprintf(&b, "%02x", a[i])
	}
	return b.String()
}

// Key is a link key: the shared secret that lets a phone reconnect without pairing again.
type Key [16]byte

// Event is one HCI event, after the transport has found its edges.
type Event struct {
	Code   byte
	Params []byte
}

// Addr is the address an event is about, for the events that lead with one.
func (e Event) Addr() (Addr, bool) { return ParseAddr(e.Params) }

// Command is one HCI command to send. Framing it for the wire is the transport's job.
type Command struct {
	Opcode uint16
	Params []byte
}

// String is the opcode and its parameters, for a log that has to be read later.
func (c Command) String() string {
	return fmt.Sprintf("%#04x %x", c.Opcode, c.Params)
}

// reply is a command whose whole payload is an address, which most of these are.
func reply(opcode uint16, a Addr) Command {
	return Command{Opcode: opcode, Params: append([]byte(nil), a[:]...)}
}

// Connected is which device a connection complete is about, and whether it connected.
//
// Its address is behind a status and a handle, where a connection request leads with one.
func Connected(params []byte) (Addr, bool) {
	const at = 1 + 2
	if len(params) < at+6 || params[0] != 0 {
		return Addr{}, false
	}
	return ParseAddr(params[at:])
}

// RemoteName asks what a device calls itself.
//
// The rest of the parameters are how to find it if it is not already connected: a page scan
// repetition mode, a reserved byte and a clock offset. Zeroes do for a device on the other end of a
// live link, which is the only one this asks about.
func RemoteName(a Addr) Command {
	params := append([]byte(nil), a[:]...)
	return Command{Opcode: OpRemoteName, Params: append(params, 0x01, 0x00, 0x00, 0x00)}
}

// ParseRemoteName reads the answer: which device, and what it calls itself.
//
// The name is 248 bytes however long it is, padded with zeroes.
func ParseRemoteName(params []byte) (Addr, string, error) {
	const size = 1 + 6 + 248
	if len(params) < 7 {
		return Addr{}, "", fmt.Errorf("pair: a remote name of %d bytes", len(params))
	}

	if status := params[0]; status != 0 {
		return Addr{}, "", fmt.Errorf("pair: the far end would not give its name: %#02x", status)
	}

	a, ok := ParseAddr(params[1:])
	if !ok {
		return Addr{}, "", fmt.Errorf("pair: a remote name with no address")
	}

	name := params[7:]
	if len(params) > size {
		name = params[7:size]
	}
	return a, string(bytes.TrimRight(name, "\x00")), nil
}

// Handle reads a connection handle out of an event that leads with a status and one.
//
// The top four bits are reserved and are not part of the handle. Leaving them in gives a number
// that matches nothing, which shows up as a link that is up and cannot be written to.
func Handle(params []byte) (uint16, bool) {
	if len(params) < 3 {
		return 0, false
	}
	return binary.LittleEndian.Uint16(params[1:]) & 0x0fff, true
}
