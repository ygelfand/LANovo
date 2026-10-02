package pair

import (
	"encoding/binary"
	"fmt"
	"maps"
	"slices"
)

// Store is where link keys live between boots.
//
// A key not written down is a phone that pairs again every time it connects, which is the failure
// people actually notice. An in-memory one is fine for a test and wrong for the device.
type Store interface {
	// Key is the one saved for an address, and whether there is one.
	Key(Addr) (Key, bool)

	// Save records a key, replacing whatever was there. A phone that re-pairs sends a new one.
	Save(Addr, Key) error

	// Forget drops it, for a phone that has been told to unpair.
	Forget(Addr) error
}

// Memory is a Store that remembers nothing past this process, for tests and for a device that has
// not been given anywhere to write yet.
type Memory map[Addr]Key

func (m Memory) Key(a Addr) (Key, bool) { k, ok := m[a]; return k, ok }

func (m Memory) Save(a Addr, k Key) error {
	m[a] = k
	return nil
}

func (m Memory) Forget(a Addr) error {
	delete(m, a)
	return nil
}

// Peer is one phone and where it has got to.
type Peer struct {
	Addr Addr

	// Handle is the connection, once there is one. Zero is a valid handle, so Connected says
	// whether this means anything.
	Handle    uint16
	Connected bool

	// Bonded is a link key having been agreed, whether now or on some previous day.
	Bonded bool

	// Encrypted is the link actually being encrypted, which is a separate step the phone asks for
	// after authenticating.
	Encrypted bool
}

// Manager answers the HCI events that open a link and bond with a phone.
//
// Not safe for concurrent use: it is driven from whichever goroutine reads the controller, and a
// lock here would only hide that a second caller has no ordering to rely on.
type Manager struct {
	// Keys is where link keys are kept. Nil means nothing is remembered and every connection pairs
	// again, which works and is not what anyone wants.
	Keys Store

	// IO is what this device can do about confirming a pairing. NoInputNoOutput unless something
	// sets otherwise, which is Just Works and what a speaker does.
	IO byte

	// Auth is what the pairing has to achieve. General bonding, or no key is generated.
	Auth byte

	// Accept decides whether to let a phone connect at all. Nil accepts everything, which is what
	// a speaker in a room does. A device in pairing mode only would say so here.
	Accept func(Addr) bool

	// Confirm answers numeric comparison, for an IO capability that has a screen. Nil confirms
	// everything, which is the only answer Just Works has.
	Confirm func(Addr, uint32) bool

	// Bonded and Lost are called as a phone finishes pairing and as its link goes away.
	Bonded func(*Peer)
	Lost   func(*Peer)

	peers map[Addr]*Peer
}

// New is a manager that behaves the way a speaker does: accepts anyone, asks nobody.
func New(keys Store) *Manager {
	if keys == nil {
		keys = Memory{}
	}
	return &Manager{
		Keys:  keys,
		IO:    NoInputNoOutput,
		Auth:  GeneralBonding,
		peers: map[Addr]*Peer{},
	}
}

// Peer is what is known about an address, or nil.
func (m *Manager) Peer(a Addr) *Peer { return m.peers[a] }

// Peers is everything currently known, in address order so a caller can print it.
func (m *Manager) Peers() []*Peer {
	out := slices.Collect(maps.Values(m.peers))
	slices.SortFunc(out, func(a, b *Peer) int {
		return slices.Compare(a.Addr[:], b.Addr[:])
	})
	return out
}

// Connected is the peer on a connection handle, or nil. The handle is what everything above this
// addresses a link by.
func (m *Manager) Connected(handle uint16) *Peer {
	for _, p := range m.peers {
		if p.Connected && p.Handle == handle {
			return p
		}
	}
	return nil
}

func (m *Manager) peer(a Addr) *Peer {
	p := m.peers[a]
	if p == nil {
		p = &Peer{Addr: a}
		m.peers[a] = p
	}
	return p
}

// Handle answers one event with the commands to send.
//
// An event this does not care about produces nothing and no error: most of what a controller sends
// belongs to somebody else, and treating that as a fault would make every link look broken.
//
// What is answered is answered. A controller left waiting on a reply to a request stalls the whole
// pairing, and the phone gives up with no reason shown.
func (m *Manager) Handle(e Event) ([]Command, error) {
	switch e.Code {
	case EventConnectionRequest:
		return m.requested(e)
	case EventConnectionComplete:
		return nil, m.connected(e)
	case EventDisconnectionComplete:
		return nil, m.disconnected(e)
	case EventLinkKeyRequest:
		return m.linkKey(e)
	case EventLinkKeyNotification:
		return nil, m.remember(e)
	case EventIOCapabilityRequest:
		return m.capability(e)
	case EventUserConfirmation:
		return m.confirm(e)
	case EventPINCodeRequest:
		return m.legacy(e)
	case EventUserPasskeyRequest:
		return m.passkey(e)
	case EventSimplePairingComplete:
		return nil, m.paired(e)
	case EventEncryptionChange:
		return nil, m.encrypted(e)
	}
	return nil, nil
}

// requested answers a phone asking to connect.
func (m *Manager) requested(e Event) ([]Command, error) {
	// Address, class of device, link type.
	if len(e.Params) < 10 {
		return nil, fmt.Errorf("pair: a connection request of %d bytes", len(e.Params))
	}

	a, _ := ParseAddr(e.Params)

	// Only ACL. A phone asking for SCO here is asking for a voice call, which is a different
	// profile and is refused rather than accepted and left silent.
	if link := e.Params[9]; link != LinkACL {
		return []Command{rejectWith(a, RejectUnacceptableAddr)}, nil
	}

	if m.Accept != nil && !m.Accept(a) {
		return []Command{rejectWith(a, RejectSecurity)}, nil
	}

	m.peer(a)

	// Role 0x01 is staying the peripheral. A speaker has no reason to take over timing for the
	// link, and asking to becomes a role switch the phone may refuse mid-connection.
	return []Command{{
		Opcode: OpAcceptConnection,
		Params: append(append([]byte(nil), a[:]...), 0x01),
	}}, nil
}

func rejectWith(a Addr, reason byte) Command {
	return Command{
		Opcode: OpRejectConnection,
		Params: append(append([]byte(nil), a[:]...), reason),
	}
}

// connected records a link that came up, and forgets one that failed to.
func (m *Manager) connected(e Event) error {
	// Status, handle, address, link type, encryption mode.
	if len(e.Params) < 11 {
		return fmt.Errorf("pair: a connection complete of %d bytes", len(e.Params))
	}

	a, _ := ParseAddr(e.Params[3:])

	if status := e.Params[0]; status != 0 {
		delete(m.peers, a)
		return fmt.Errorf("pair: %v did not connect, status %#02x", a, status)
	}

	p := m.peer(a)
	p.Handle = binary.LittleEndian.Uint16(e.Params[1:]) & 0x0fff
	p.Connected = true
	p.Encrypted = false
	return nil
}

// disconnected clears what was true only while the link was up.
//
// The bond is not one of those things. A phone that walks out of the room is still paired when it
// walks back in, which is the whole point of having saved a key.
func (m *Manager) disconnected(e Event) error {
	handle, ok := Handle(e.Params)
	if !ok {
		return fmt.Errorf("pair: a disconnection complete of %d bytes", len(e.Params))
	}

	p := m.Connected(handle)
	if p == nil {
		return nil
	}

	p.Connected = false
	p.Encrypted = false
	if m.Lost != nil {
		m.Lost(p)
	}
	return nil
}

// linkKey answers the controller asking whether this phone is known.
func (m *Manager) linkKey(e Event) ([]Command, error) {
	a, ok := e.Addr()
	if !ok {
		return nil, fmt.Errorf("pair: a link key request of %d bytes", len(e.Params))
	}

	key, ok := m.Keys.Key(a)
	if !ok {
		// Saying no is what starts a pairing. Silence here is a phone that waits and then gives up.
		m.peer(a).Bonded = false
		return []Command{reply(OpLinkKeyNegative, a)}, nil
	}

	m.peer(a).Bonded = true
	return []Command{{
		Opcode: OpLinkKeyReply,
		Params: append(append([]byte(nil), a[:]...), key[:]...),
	}}, nil
}

// remember saves the key a pairing produced.
func (m *Manager) remember(e Event) error {
	// Address, key, key type.
	if len(e.Params) < 23 {
		return fmt.Errorf("pair: a link key notification of %d bytes", len(e.Params))
	}

	a, _ := ParseAddr(e.Params)

	// The debug key is a fixed value published in the spec so that traffic can be decrypted during
	// development. Saving it would leave the link readable by anyone who knows it, which is
	// everyone — so it is used for this connection and never written down.
	if e.Params[22] == KeyDebug {
		return fmt.Errorf("pair: %v produced a debug key, which is not saved", a)
	}

	var key Key
	copy(key[:], e.Params[6:22])

	if err := m.Keys.Save(a, key); err != nil {
		return fmt.Errorf("pair: saving the key for %v: %w", a, err)
	}

	m.peer(a).Bonded = true
	return nil
}

// capability answers what this device can do about confirming a pairing.
func (m *Manager) capability(e Event) ([]Command, error) {
	a, ok := e.Addr()
	if !ok {
		return nil, fmt.Errorf("pair: an io capability request of %d bytes", len(e.Params))
	}

	// Capability, out of band data present, authentication requirements.
	return []Command{{
		Opcode: OpIOCapabilityReply,
		Params: append(append([]byte(nil), a[:]...), m.IO, 0x00, m.Auth),
	}}, nil
}

// confirm answers numeric comparison.
//
// With NoInputNoOutput there is nobody to ask and the number is not shown anywhere, so this is
// where Just Works quietly says yes. That is not a shortcut in the implementation — it is what the
// pairing is, and it is why Just Works has no protection against someone in the middle.
func (m *Manager) confirm(e Event) ([]Command, error) {
	a, ok := e.Addr()
	if !ok {
		return nil, fmt.Errorf("pair: a user confirmation request of %d bytes", len(e.Params))
	}

	var number uint32
	if len(e.Params) >= 10 {
		number = binary.LittleEndian.Uint32(e.Params[6:])
	}

	if m.Confirm != nil && !m.Confirm(a, number) {
		return []Command{reply(OpConfirmNegative, a)}, nil
	}
	return []Command{reply(OpConfirmReply, a)}, nil
}

// legacy refuses PIN pairing.
//
// Everything since about 2007 does secure simple pairing, and answering this would mean having a
// PIN for somebody to type into a device with no keyboard. Refusing makes the phone fall back to
// the pairing it can do, where silence makes it hang.
func (m *Manager) legacy(e Event) ([]Command, error) {
	a, ok := e.Addr()
	if !ok {
		return nil, fmt.Errorf("pair: a pin code request of %d bytes", len(e.Params))
	}
	return []Command{reply(OpPINCodeNegative, a)}, nil
}

// passkey refuses to be typed into.
//
// A passkey request means the phone thinks this device has a keyboard. It does not, whatever it
// declared, so this is refused rather than answered with a guess.
func (m *Manager) passkey(e Event) ([]Command, error) {
	a, ok := e.Addr()
	if !ok {
		return nil, fmt.Errorf("pair: a passkey request of %d bytes", len(e.Params))
	}
	return []Command{reply(OpPasskeyNegative, a)}, nil
}

// paired is the pairing having finished, one way or the other.
func (m *Manager) paired(e Event) error {
	// Status, then the address.
	if len(e.Params) < 7 {
		return fmt.Errorf("pair: a simple pairing complete of %d bytes", len(e.Params))
	}

	a, _ := ParseAddr(e.Params[1:])

	if status := e.Params[0]; status != 0 {
		// The key, if one was saved, is no good. Keeping it means every future connection offers a
		// key the phone will reject, and the phone never gets the chance to pair again.
		p := m.peer(a)
		p.Bonded = false
		if err := m.Keys.Forget(a); err != nil {
			return fmt.Errorf("pair: dropping the key for %v: %w", a, err)
		}
		return fmt.Errorf("pair: %v failed to pair, status %#02x", a, status)
	}

	p := m.peer(a)
	p.Bonded = true
	if m.Bonded != nil {
		m.Bonded(p)
	}
	return nil
}

// encrypted records the link being encrypted, or stopping being.
func (m *Manager) encrypted(e Event) error {
	// Status, handle, enabled.
	if len(e.Params) < 4 {
		return fmt.Errorf("pair: an encryption change of %d bytes", len(e.Params))
	}

	handle, _ := Handle(e.Params)
	p := m.Connected(handle)
	if p == nil {
		return nil
	}

	if status := e.Params[0]; status != 0 {
		p.Encrypted = false
		return fmt.Errorf("pair: encryption for %v failed, status %#02x", p.Addr, status)
	}

	p.Encrypted = e.Params[3] != 0
	return nil
}

// Forget drops a phone entirely, key included, for one that has been unpaired at the other end or
// in a setting here.
func (m *Manager) Forget(a Addr) error {
	delete(m.peers, a)
	return m.Keys.Forget(a)
}
