package pair

import (
	"errors"
	"testing"
)

var device = Addr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}

// An address is stored the way it goes on the wire and printed the way people write it. Getting
// that backwards makes a phone that paired yesterday a stranger today.
func TestAnAddressPrintsTheWayItIsWritten(t *testing.T) {
	if got := device.String(); got != "66:55:44:33:22:11" {
		t.Errorf("came back %q", got)
	}
}

func TestAnAddressTooShortToBeOne(t *testing.T) {
	if _, ok := ParseAddr(make([]byte, 5)); ok {
		t.Error("five bytes were read as an address")
	}
}

// only is the one command an event should have produced.
func only(t *testing.T, m *Manager, e Event) Command {
	t.Helper()

	out, err := m.Handle(e)
	if err != nil {
		t.Fatalf("event %#02x: %v", e.Code, err)
	}
	if len(out) != 1 {
		t.Fatalf("event %#02x gave %d commands, want one", e.Code, len(out))
	}
	return out[0]
}

// quiet runs an event that should produce nothing and should not fail.
func quiet(t *testing.T, m *Manager, e Event) {
	t.Helper()

	out, err := m.Handle(e)
	if err != nil {
		t.Fatalf("event %#02x: %v", e.Code, err)
	}
	if len(out) != 0 {
		t.Fatalf("event %#02x gave %d commands, want none", e.Code, len(out))
	}
}

func connectionRequest(a Addr, link byte) Event {
	params := append(append([]byte(nil), a[:]...), 0x24, 0x04, 0x20, link)
	return Event{Code: EventConnectionRequest, Params: params}
}

func connectionComplete(a Addr, handle uint16, status byte) Event {
	params := []byte{status, byte(handle), byte(handle >> 8)}
	params = append(params, a[:]...)
	return Event{Code: EventConnectionComplete, Params: append(params, LinkACL, 0x00)}
}

func notification(a Addr, key Key, kind byte) Event {
	params := append(append([]byte(nil), a[:]...), key[:]...)
	return Event{Code: EventLinkKeyNotification, Params: append(params, kind)}
}

// The whole thing, in the order a controller sends it: a phone nobody has seen before connects,
// pairs, and is remembered.
func TestAPhoneConnectsAndIsRemembered(t *testing.T) {
	keys := Memory{}
	m := New(keys)

	var bonded int
	m.Bonded = func(*Peer) { bonded++ }

	// It asks to connect and is let in as the peripheral.
	accept := only(t, m, connectionRequest(device, LinkACL))
	if accept.Opcode != OpAcceptConnection {
		t.Fatalf("answered a connection request with %#04x", accept.Opcode)
	}
	if role := accept.Params[6]; role != 0x01 {
		t.Errorf("accepted with role %#02x, want to stay the peripheral", role)
	}

	quiet(t, m, connectionComplete(device, 0x0c, 0))

	p := m.Peer(device)
	if p == nil || !p.Connected {
		t.Fatal("the peer did not come up")
	}
	if p.Handle != 0x0c {
		t.Errorf("handle %#x, want 0x0c", p.Handle)
	}

	// Never seen before, so there is no key and saying so is what starts the pairing.
	negative := only(t, m, Event{Code: EventLinkKeyRequest, Params: device[:]})
	if negative.Opcode != OpLinkKeyNegative {
		t.Fatalf("answered a link key request for an unknown phone with %#04x", negative.Opcode)
	}

	// What can this device do about confirming? Nothing, so Just Works.
	capability := only(t, m, Event{Code: EventIOCapabilityRequest, Params: device[:]})
	if capability.Opcode != OpIOCapabilityReply {
		t.Fatalf("answered an io capability request with %#04x", capability.Opcode)
	}
	if got := capability.Params[6]; got != NoInputNoOutput {
		t.Errorf("declared capability %#02x, want no input and no output", got)
	}
	if got := capability.Params[8]; got != GeneralBonding {
		t.Errorf("asked for %#02x, want general bonding or no key is generated", got)
	}

	// The number nobody sees.
	confirm := only(t, m, Event{
		Code:   EventUserConfirmation,
		Params: append(append([]byte(nil), device[:]...), 0x39, 0x30, 0x00, 0x00),
	})
	if confirm.Opcode != OpConfirmReply {
		t.Fatalf("answered a confirmation request with %#04x", confirm.Opcode)
	}

	// The key, which has to be written down.
	key := Key{9: 0xab}
	quiet(t, m, notification(device, key, KeyUnauthenticatedP256))
	quiet(t, m, Event{
		Code:   EventSimplePairingComplete,
		Params: append([]byte{0x00}, device[:]...),
	})

	if bonded != 1 {
		t.Errorf("bonded fired %d times", bonded)
	}
	if !m.Peer(device).Bonded {
		t.Error("the peer is not bonded")
	}

	saved, ok := keys.Key(device)
	if !ok {
		t.Fatal("no key was saved, so this phone pairs again every boot")
	}
	if saved != key {
		t.Errorf("the key came back %x", saved)
	}
}

// The second time it connects there is a key, and offering it is what makes reconnecting silent.
func TestAKnownPhoneIsNotAskedToPairAgain(t *testing.T) {
	key := Key{0: 0x01, 15: 0xff}
	m := New(Memory{device: key})

	answer := only(t, m, Event{Code: EventLinkKeyRequest, Params: device[:]})
	if answer.Opcode != OpLinkKeyReply {
		t.Fatalf("answered with %#04x, want the stored key", answer.Opcode)
	}
	if len(answer.Params) != 22 {
		t.Fatalf("the reply is %d bytes, want an address and a key", len(answer.Params))
	}
	if got := Key(answer.Params[6:22]); got != key {
		t.Errorf("offered %x, want %x", got, key)
	}
	if !m.Peer(device).Bonded {
		t.Error("a phone whose key was offered is not marked bonded")
	}
}

// The debug key is a value published in the spec so traffic can be decrypted during development.
// Saving it leaves the link readable by anyone who knows it, which is everyone.
func TestTheDebugKeyIsNotSaved(t *testing.T) {
	keys := Memory{}
	m := New(keys)

	if _, err := m.Handle(notification(device, Key{}, KeyDebug)); err == nil {
		t.Error("a debug key was accepted quietly")
	}
	if _, ok := keys.Key(device); ok {
		t.Error("a debug key was written down")
	}
}

// A pairing that fails leaves no key. Keeping one means every future connection offers something
// the phone rejects, and it never gets the chance to pair again.
func TestAFailedPairingDropsTheKey(t *testing.T) {
	keys := Memory{device: Key{0: 0xaa}}
	m := New(keys)

	_, err := m.Handle(Event{
		Code:   EventSimplePairingComplete,
		Params: append([]byte{0x05}, device[:]...),
	})
	if err == nil {
		t.Error("a failed pairing reported success")
	}
	if _, ok := keys.Key(device); ok {
		t.Error("the key survived a failed pairing")
	}
	if m.Peer(device).Bonded {
		t.Error("the peer is still marked bonded")
	}
}

// Walking out of the room is not unpairing. The bond is what makes walking back in silent.
func TestDisconnectingDoesNotUnpair(t *testing.T) {
	keys := Memory{}
	m := New(keys)

	var lost int
	m.Lost = func(*Peer) { lost++ }

	m.Handle(connectionRequest(device, LinkACL))
	quiet(t, m, connectionComplete(device, 0x0c, 0))
	quiet(t, m, notification(device, Key{0: 0x7f}, KeyAuthenticatedP256))

	quiet(t, m, Event{Code: EventDisconnectionComplete, Params: []byte{0x00, 0x0c, 0x00, 0x13}})

	if lost != 1 {
		t.Errorf("lost fired %d times", lost)
	}

	p := m.Peer(device)
	if p.Connected {
		t.Error("the peer is still connected")
	}
	if !p.Bonded {
		t.Error("disconnecting unpaired the phone")
	}
	if _, ok := keys.Key(device); !ok {
		t.Error("disconnecting dropped the key")
	}
}

// A connection that never came up leaves nothing behind.
func TestAConnectionThatFailed(t *testing.T) {
	m := New(nil)

	if _, err := m.Handle(connectionComplete(device, 0x0c, 0x04)); err == nil {
		t.Error("a failed connection reported success")
	}
	if m.Peer(device) != nil {
		t.Error("a peer was left behind by a connection that never came up")
	}
}

// SCO is a voice call, which is a different profile. Accepting it gives a link that carries
// nothing this device knows how to answer.
func TestASCOConnectionIsRefused(t *testing.T) {
	m := New(nil)

	answer := only(t, m, connectionRequest(device, LinkSCO))
	if answer.Opcode != OpRejectConnection {
		t.Fatalf("a sco request was answered with %#04x", answer.Opcode)
	}
}

func TestAConnectionCanBeRefused(t *testing.T) {
	m := New(nil)
	m.Accept = func(Addr) bool { return false }

	answer := only(t, m, connectionRequest(device, LinkACL))
	if answer.Opcode != OpRejectConnection {
		t.Fatalf("answered with %#04x, want a refusal", answer.Opcode)
	}
	if reason := answer.Params[6]; reason != RejectSecurity {
		t.Errorf("refused with %#02x", reason)
	}
	if m.Peer(device) != nil {
		t.Error("a refused phone was remembered")
	}
}

// With a screen this device could do numeric comparison, which is the difference between Just
// Works and a link that is actually authenticated.
func TestNumericComparisonCanBeRefused(t *testing.T) {
	m := New(nil)
	m.IO = DisplayYesNo

	var shown uint32
	m.Confirm = func(_ Addr, number uint32) bool {
		shown = number
		return false
	}

	answer := only(t, m, Event{
		Code:   EventUserConfirmation,
		Params: append(append([]byte(nil), device[:]...), 0x39, 0x30, 0x00, 0x00),
	})
	if answer.Opcode != OpConfirmNegative {
		t.Fatalf("a refused comparison was answered with %#04x", answer.Opcode)
	}
	if shown != 0x3039 {
		t.Errorf("the number shown was %d, want %d", shown, 0x3039)
	}

	capability := only(t, m, Event{Code: EventIOCapabilityRequest, Params: device[:]})
	if got := capability.Params[6]; got != DisplayYesNo {
		t.Errorf("declared %#02x, want the one with a screen", got)
	}
}

// Both of these mean the phone thinks somebody can type. Refusing makes it fall back to what it
// can do; silence makes it hang.
func TestLegacyPairingAndPasskeysAreRefusedRatherThanIgnored(t *testing.T) {
	m := New(nil)

	if got := only(t, m, Event{Code: EventPINCodeRequest, Params: device[:]}); got.Opcode != OpPINCodeNegative {
		t.Errorf("a pin request was answered with %#04x", got.Opcode)
	}
	if got := only(t, m, Event{Code: EventUserPasskeyRequest, Params: device[:]}); got.Opcode != OpPasskeyNegative {
		t.Errorf("a passkey request was answered with %#04x", got.Opcode)
	}
}

func TestEncryptionIsTracked(t *testing.T) {
	m := New(nil)
	m.Handle(connectionRequest(device, LinkACL))
	quiet(t, m, connectionComplete(device, 0x0c, 0))

	quiet(t, m, Event{Code: EventEncryptionChange, Params: []byte{0x00, 0x0c, 0x00, 0x01}})
	if !m.Peer(device).Encrypted {
		t.Error("the link is not marked encrypted")
	}

	quiet(t, m, Event{Code: EventEncryptionChange, Params: []byte{0x00, 0x0c, 0x00, 0x00}})
	if m.Peer(device).Encrypted {
		t.Error("encryption being turned off was not noticed")
	}
}

// The top four bits of a handle are reserved. Leaving them in gives a number matching no link.
func TestTheReservedBitsOfAHandleAreMasked(t *testing.T) {
	m := New(nil)
	m.Handle(connectionRequest(device, LinkACL))

	quiet(t, m, connectionComplete(device, 0xf00c, 0))

	if got := m.Peer(device).Handle; got != 0x0c {
		t.Errorf("handle %#x, want the reserved bits gone", got)
	}
	if m.Connected(0x0c) == nil {
		t.Error("the peer cannot be found by its handle")
	}
}

// An event for a link that is not ours is not an error. Most of what a controller sends belongs to
// somebody else, and treating that as a fault makes every link look broken.
func TestAnEventForSomebodyElse(t *testing.T) {
	m := New(nil)

	quiet(t, m, Event{Code: EventEncryptionChange, Params: []byte{0x00, 0x33, 0x00, 0x01}})
	quiet(t, m, Event{Code: EventDisconnectionComplete, Params: []byte{0x00, 0x33, 0x00, 0x13}})
	quiet(t, m, Event{Code: 0x22, Params: []byte{1, 2, 3}})
}

// Every event that leads with an address is answered, so a truncated one has to be caught rather
// than read past the end.
func TestTruncatedEventsAreRefused(t *testing.T) {
	m := New(nil)

	for _, code := range []byte{
		EventConnectionRequest,
		EventConnectionComplete,
		EventDisconnectionComplete,
		EventLinkKeyRequest,
		EventLinkKeyNotification,
		EventIOCapabilityRequest,
		EventUserConfirmation,
		EventPINCodeRequest,
		EventUserPasskeyRequest,
		EventSimplePairingComplete,
		EventEncryptionChange,
	} {
		if _, err := m.Handle(Event{Code: code, Params: []byte{0x01, 0x02}}); err == nil {
			t.Errorf("event %#02x was accepted with two bytes of parameters", code)
		}
	}
}

// A store that cannot write is a device with a read-only filesystem, which has to be reported
// rather than leaving the caller thinking a phone was remembered.
func TestAStoreThatCannotSaveIsReported(t *testing.T) {
	m := New(broken{})

	if _, err := m.Handle(notification(device, Key{}, KeyUnauthenticated)); err == nil {
		t.Error("a key that could not be saved reported success")
	}
}

type broken struct{}

var errBroken = errors.New("nowhere to write")

func (broken) Key(Addr) (Key, bool) { return Key{}, false }
func (broken) Save(Addr, Key) error { return errBroken }
func (broken) Forget(Addr) error    { return errBroken }

func TestForgettingAPhone(t *testing.T) {
	keys := Memory{device: Key{0: 1}}
	m := New(keys)
	m.Handle(connectionRequest(device, LinkACL))

	if err := m.Forget(device); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if m.Peer(device) != nil {
		t.Error("the peer survived")
	}
	if _, ok := keys.Key(device); ok {
		t.Error("the key survived")
	}
}

func TestPeersComeBackInAStableOrder(t *testing.T) {
	m := New(nil)
	other := Addr{0x01}

	m.Handle(connectionRequest(other, LinkACL))
	m.Handle(connectionRequest(device, LinkACL))

	got := m.Peers()
	if len(got) != 2 {
		t.Fatalf("%d peers", len(got))
	}
	if got[0].Addr != other {
		t.Errorf("came back starting with %v", got[0].Addr)
	}
}

// A phone is asked what it calls itself, and the answer is 248 bytes however short the name.
func TestAskingWhatAPhoneIsCalled(t *testing.T) {
	a := Addr{1, 2, 3, 4, 5, 6}

	c := RemoteName(a)
	if c.Opcode != OpRemoteName {
		t.Fatalf("opcode %#04x, want %#04x", c.Opcode, OpRemoteName)
	}
	if len(c.Params) != 10 {
		t.Fatalf("%d parameters, want the address and four more", len(c.Params))
	}
	if got, _ := ParseAddr(c.Params); got != a {
		t.Errorf("addressed to %v, want %v", got, a)
	}

	params := append([]byte{0x00}, a[:]...)
	params = append(params, "Pixel 9"...)
	params = append(params, make([]byte, 248-len("Pixel 9"))...)

	who, name, err := ParseRemoteName(params)
	if err != nil {
		t.Fatalf("ParseRemoteName: %v", err)
	}
	if who != a {
		t.Errorf("the name came from %v, want %v", who, a)
	}
	if name != "Pixel 9" {
		t.Errorf("name %q, want it without the padding", name)
	}
}

// A connection complete carries its address behind a status and a handle, where a request leads
// with one. Reading it from the front asks about six bytes of something else.
func TestWhereAConnectionCompleteCarriesItsAddress(t *testing.T) {
	a := Addr{1, 2, 3, 4, 5, 6}

	params := []byte{0x00, 0x0c, 0x00}
	params = append(params, a[:]...)
	params = append(params, LinkACL, 0x00)

	got, ok := Connected(params)
	if !ok {
		t.Fatal("a connection complete read as no connection")
	}
	if got != a {
		t.Errorf("connected to %v, want %v", got, a)
	}

	params[0] = 0x04
	if _, ok := Connected(params); ok {
		t.Error("a connection that failed read as one that worked")
	}
}

func TestAPhoneThatWillNotSayItsName(t *testing.T) {
	if _, _, err := ParseRemoteName([]byte{0x04, 1, 2, 3, 4, 5, 6}); err == nil {
		t.Error("a refusal read as a name")
	}
	if _, _, err := ParseRemoteName([]byte{0x00, 1, 2}); err == nil {
		t.Error("a truncated answer read as a name")
	}
}
