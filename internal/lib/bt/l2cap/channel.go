package l2cap

import "fmt"

// Channels, and the state machine that opens and closes them.
//
// A phone that wants to play music opens two: one to SDP to find out what this device is, then one
// to AVDTP to negotiate and stream. Both arrive as connection requests we answer, so for audio this
// side is the responder throughout.
//
// Not for everything, though. The half of remote control that says which player is addressed, and
// the channel cover art is fetched over, are both the controller's to open — and this device is the
// controller. So a channel can start at either end, and the two paths meet at the same
// configuration round.

// State is where a channel is in its life.
type State int

const (
	// Closed is a channel that does not exist, which is what an unknown number reports as.
	Closed State = iota

	// Connecting is a request of ours that has not been answered. Only a channel this side opened
	// is ever here; one that arrives already has the far end's number.
	Connecting

	// Configuring is connected but not usable: both ends have to finish a configuration round
	// before anything may be sent, and either may still refuse.
	Configuring

	// Open is both rounds done. Data may flow.
	Open

	// Closing is a disconnection in flight.
	Closing
)

func (s State) String() string {
	switch s {
	case Connecting:
		return "connecting"
	case Configuring:
		return "configuring"
	case Open:
		return "open"
	case Closing:
		return "closing"
	}
	return "closed"
}

// Channel is one open path to the far end.
type Channel struct {
	// Local is the number we know it by and Remote the number they do. They are different and
	// neither side may assume the other's.
	Local  uint16
	Remote uint16

	PSM   uint16
	State State

	// Ours says this end opened it. Two channels to the same protocol, one each way, are told apart
	// by nothing else.
	Ours bool

	// MTU is the largest packet the far end said it will accept, which is what our own writes are
	// fragmented against. Absent from their configuration means the default rather than nothing.
	MTU uint16

	// Both ends configure independently and a channel is not open until both have finished.
	theirs bool
	ours   bool
}

// Manager holds the channels and answers the signalling that opens them.
//
// Not safe for concurrent use: it is driven from whichever goroutine reads the radio, and a lock
// here would only hide that a second caller has no ordering to rely on anyway.
type Manager struct {
	channels map[uint16]*Channel
	serving  map[uint16]bool

	// asking is the channel each outstanding request of ours is for, by the identifier it was sent
	// with. A response carries our number back, but only the identifier says which request it
	// answers when several are in flight.
	asking map[byte]uint16

	next uint16
	id   byte

	// mtu is what we tell the far end we will accept.
	mtu uint16
}

// NewManager is a manager serving nothing yet.
func NewManager(mtu uint16) *Manager {
	if mtu == 0 {
		mtu = MTUDefault
	}
	return &Manager{
		channels: map[uint16]*Channel{},
		serving:  map[uint16]bool{},
		asking:   map[byte]uint16{},
		next:     CIDDynamic,
		mtu:      mtu,
	}
}

// Connect opens a channel to the far end, and is the request to send.
//
// The channel exists here from this moment, in Connecting, so a second call for the same protocol
// does not allocate a second one. It is not usable until the far end has answered and both ends
// have configured; Channel reports where it has got to.
func (m *Manager) Connect(psm uint16) (Command, error) {
	for _, c := range m.channels {
		if c.PSM == psm && c.State != Closed && c.State != Closing {
			return Command{}, fmt.Errorf("l2cap: channel %#x to %#04x is already %s",
				c.Local, psm, c.State)
		}
	}

	local, err := m.allocate()
	if err != nil {
		return Command{}, err
	}

	m.channels[local] = &Channel{
		Local: local,
		PSM:   psm,
		State: Connecting,
		MTU:   MTUDefault,
		Ours:  true,
	}

	id := m.nextID()
	m.asking[id] = local

	return Connect{PSM: psm, SourceCID: local}.Command(id), nil
}

// Serve says this device answers connections to a protocol. Anything else is refused with the
// reason that says so, rather than ignored: a phone told no moves on, a phone told nothing waits.
func (m *Manager) Serve(psm uint16) { m.serving[psm] = true }

// Channel is the one with this local number, or nil.
func (m *Manager) Channel(cid uint16) *Channel { return m.channels[cid] }

// Open is every channel that is usable, for a caller deciding whether it has what it needs.
func (m *Manager) Open() []*Channel {
	var out []*Channel
	for _, c := range m.channels {
		if c.State == Open {
			out = append(out, c)
		}
	}
	return out
}

// allocate hands out the next free local channel number.
//
// Dynamic numbers start at 0x0040 and wrap; below that is reserved. A device with sixty thousand
// channels open is broken rather than busy, so running out is an error and not a wait.
func (m *Manager) allocate() (uint16, error) {
	for range 0xffff - CIDDynamic {
		cid := m.next
		m.next++
		if m.next < CIDDynamic {
			m.next = CIDDynamic
		}

		if m.channels[cid] == nil {
			return cid, nil
		}
	}
	return 0, fmt.Errorf("l2cap: no channel numbers left")
}

// nextID is the identifier for a request of our own. Zero is not a valid one.
func (m *Manager) nextID() byte {
	m.id++
	if m.id == 0 {
		m.id = 1
	}
	return m.id
}

// Handle takes one frame off the signalling channel and answers with what to send.
//
// Everything is answered, including what cannot be understood. A far end waiting on a reply it will
// never get is worse than one told no.
func (m *Manager) Handle(f Frame) ([]Command, error) {
	if f.CID != CIDSignalling {
		return nil, fmt.Errorf("l2cap: %#x is not the signalling channel", f.CID)
	}

	commands, err := ParseCommands(f.Payload)
	if err != nil {
		return nil, err
	}

	var out []Command
	for _, c := range commands {
		answers, err := m.one(c)
		if err != nil {
			return out, err
		}
		out = append(out, answers...)
	}
	return out, nil
}

func (m *Manager) one(c Command) ([]Command, error) {
	switch c.Code {
	case CodeConnectRequest:
		return m.connect(c)
	case CodeConfigRequest:
		return m.configure(c)
	case CodeConfigResponse:
		return m.configured(c)
	case CodeDisconnectRequest:
		return m.disconnect(c)
	case CodeEchoRequest:
		return []Command{{Code: CodeEchoResponse, ID: c.ID}}, nil

	case CodeInfoRequest:
		// Answered with "not supported", which is a legal answer and the honest one: nothing here
		// has extended features or a fixed channel list worth reporting.
		if len(c.Data) < 2 {
			return []Command{Reject(c.ID, RejectNotUnderstood)}, nil
		}
		data := []byte{c.Data[0], c.Data[1], 0x01, 0x00} // type, result: not supported
		return []Command{{Code: CodeInfoResponse, ID: c.ID, Data: data}}, nil

	case CodeConnectResponse:
		return m.connected(c)

	case CodeDisconnectResponse, CodeEchoResponse, CodeInfoResponse:
		// Answers to things nothing here asks for. Unexpected rather than wrong, and silence is the
		// right response to a response.
		return nil, nil
	}

	return []Command{Reject(c.ID, RejectNotUnderstood)}, nil
}

func (m *Manager) connect(c Command) ([]Command, error) {
	req, err := ParseConnect(c)
	if err != nil {
		return []Command{Reject(c.ID, RejectNotUnderstood)}, nil
	}

	if !m.serving[req.PSM] {
		return []Command{Connected{
			SourceCID: req.SourceCID,
			Result:    ConnectBadPSM,
		}.Command(c.ID)}, nil
	}

	local, err := m.allocate()
	if err != nil {
		return []Command{Connected{
			SourceCID: req.SourceCID,
			Result:    ConnectNoResources,
		}.Command(c.ID)}, nil
	}

	ch := &Channel{
		Local:  local,
		Remote: req.SourceCID,
		PSM:    req.PSM,
		State:  Configuring,
		MTU:    MTUDefault,
	}
	m.channels[local] = ch

	// Accept, then immediately ask for our own configuration. Both ends configure and the channel
	// is not open until both have finished, so waiting to be asked would leave it half open.
	return []Command{
		Connected{DestinationCID: local, SourceCID: ch.Remote, Result: ConnectSuccess}.Command(c.ID),
		Configure{
			DestinationCID: ch.Remote,
			Options:        []Option{MTUOption(m.mtu)},
		}.Command(m.nextID()),
	}, nil
}

// connected takes the answer to a connection request of ours.
//
// The two numbers in a response are the opposite way round from the one in a request: theirs is the
// destination and ours the source, because it answers a request addressed to them.
func (m *Manager) connected(c Command) ([]Command, error) {
	res, err := ParseConnected(c)
	if err != nil {
		return nil, err
	}

	local, waiting := m.asking[c.ID]
	if !waiting {
		// An answer to a request nobody made, or one already given up on. Nothing to do with it.
		return nil, nil
	}

	ch := m.channels[local]
	if ch == nil || ch.State != Connecting {
		delete(m.asking, c.ID)
		return nil, nil
	}

	// Pending is the far end asking for time, usually to put a pairing prompt on screen. The
	// request stands and another response follows, so this is not the end of anything.
	if res.Result == ConnectPending {
		return nil, nil
	}
	delete(m.asking, c.ID)

	if res.Result != ConnectSuccess {
		delete(m.channels, local)
		return nil, fmt.Errorf("l2cap: %#04x refused a channel: %#04x", ch.PSM, res.Result)
	}

	if res.SourceCID != local {
		delete(m.channels, local)
		return nil, fmt.Errorf("l2cap: an answer for channel %#x arrived on the request for %#x",
			res.SourceCID, local)
	}

	ch.Remote = res.DestinationCID
	ch.State = Configuring

	// Ask for our own configuration straight away, the same as answering a request does. Both ends
	// have to finish before anything may be sent.
	return []Command{Configure{
		DestinationCID: ch.Remote,
		Options:        []Option{MTUOption(m.mtu)},
	}.Command(m.nextID())}, nil
}

func (m *Manager) configure(c Command) ([]Command, error) {
	req, err := ParseConfigure(c)
	if err != nil {
		return []Command{Reject(c.ID, RejectNotUnderstood)}, nil
	}

	ch := m.channels[req.DestinationCID]
	if ch == nil {
		return []Command{Reject(c.ID, RejectBadCID, req.DestinationCID, 0)}, nil
	}

	// Everything here runs in basic mode. A request for another is answered with a counter-proposal
	// naming this one, since an unknown-option answer leaves a peer that requires its mode with
	// nothing to agree to.
	if mode, asked := req.Mode(); asked && mode != ModeBasic {
		return []Command{Configured{
			SourceCID: ch.Remote,
			Result:    ConfigUnacceptable,
			Options:   []Option{BasicMode()},
		}.Command(c.ID)}, nil
	}

	// An option we have to answer and cannot is refused by naming it, which is what lets the far
	// end try again without it. Hints are not in this list on purpose.
	if unknown := req.Unknown(); len(unknown) > 0 {
		options := make([]Option, 0, len(unknown))
		for _, t := range unknown {
			options = append(options, Option{Type: t})
		}
		return []Command{Configured{
			SourceCID: ch.Remote,
			Result:    ConfigUnknown,
			Options:   options,
		}.Command(c.ID)}, nil
	}

	ch.MTU = req.MTU()

	// More to come: acknowledge this part and wait rather than calling the round done.
	if req.Continues() {
		return []Command{Configured{
			SourceCID: ch.Remote,
			Flags:     ConfigFlagContinues,
			Result:    ConfigSuccess,
		}.Command(c.ID)}, nil
	}

	ch.theirs = true
	m.settle(ch)

	return []Command{Configured{
		SourceCID: ch.Remote,
		Result:    ConfigSuccess,
	}.Command(c.ID)}, nil
}

func (m *Manager) configured(c Command) ([]Command, error) {
	res, err := ParseConfigured(c)
	if err != nil {
		return nil, nil
	}

	// Our own number, not theirs. A response answers a request we addressed to their channel, and
	// names ours so we can tell which of our requests it belongs to — so this is a lookup among the
	// channels we know by that number, not a search for a far end that matches it.
	ch := m.channels[res.SourceCID]
	if ch == nil {
		return nil, nil
	}

	if res.Result != ConfigSuccess {
		// They would not have what we asked for. Nothing here has a second offer to make, so the
		// channel stays unconfigured rather than pretending.
		return nil, fmt.Errorf("l2cap: channel %#x refused our configuration: %#x",
			ch.Local, res.Result)
	}
	if res.Flags&ConfigFlagContinues != 0 {
		return nil, nil
	}

	ch.ours = true
	m.settle(ch)
	return nil, nil
}

// settle opens a channel once both ends have configured it.
func (m *Manager) settle(ch *Channel) {
	if ch.theirs && ch.ours && ch.State == Configuring {
		ch.State = Open
	}
}

func (m *Manager) disconnect(c Command) ([]Command, error) {
	req, err := ParseDisconnect(c)
	if err != nil {
		return []Command{Reject(c.ID, RejectNotUnderstood)}, nil
	}

	ch := m.channels[req.DestinationCID]
	if ch == nil {
		return []Command{Reject(c.ID, RejectBadCID, req.DestinationCID, req.SourceCID)}, nil
	}

	delete(m.channels, ch.Local)

	return []Command{Disconnect{
		DestinationCID: req.DestinationCID,
		SourceCID:      req.SourceCID,
	}.Command(CodeDisconnectResponse, c.ID)}, nil
}

// Drop forgets one channel, for a number the far end no longer knows. The far end is not told: it
// is the one that stopped answering for it.
func (m *Manager) Drop(local uint16) { delete(m.channels, local) }

// Forget drops every channel, for a link that has gone away. The far end is not told, because there
// is nothing left to tell it over.
func (m *Manager) Forget() {
	clear(m.channels)
	clear(m.asking)
}
