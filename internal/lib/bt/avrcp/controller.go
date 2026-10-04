package avrcp

import (
	"fmt"
	"log/slog"
	"time"
)

// Controller is this device driving the phone: the buttons on the screen, and keeping the screen
// up to date with what the phone is playing.
//
// Frames in, frames out, and a callback when what is on the screen should change. Nothing here
// knows about L2CAP — what comes out goes on the AVCTP channel, which the caller owns.
//
// Not safe for concurrent use: it is driven from whichever goroutine reads the channel.
type Controller struct {
	// Changed is called when the track, the progress or the volume has moved. The whole state is
	// handed over rather than a delta, because a screen redraws from what is true rather than from
	// what happened.
	Changed func(State)

	// Volume is called when the phone sets an absolute volume, for the speaker to follow. Separate
	// from Changed because this one is an instruction rather than news.
	Volume func(percent int)

	state State

	// pending is what each outstanding transaction label was asked for, so an answer can be
	// matched to its question. A phone may answer out of order and several may be in flight.
	pending map[byte]byte

	// registered is the events currently registered for. A registration is one-shot — the target
	// answers once and forgets — so this is what says which need renewing.
	registered map[byte]bool

	// registering is which event each outstanding registration is for. A rejection carries an error
	// code where the event would be, so the label is the only thing that says what was refused.
	registering map[byte]byte

	// supported is what the target said it will notify about, once it has been asked. Registering
	// for an event a phone does not support gets a rejection per event, every time.
	supported map[byte]bool
	asked     bool

	// arting is which track each outstanding artwork request was made under, and age counts the
	// tracks. An answer names no track, so a late one would land on whatever is playing when it
	// arrives.
	arting map[byte]uint32
	age    uint32

	// player is which player the far end answers for. Zero until it says: the ids a target hands
	// out start at one.
	player uint16

	// watching is the label of an outstanding volume registration, which is owed a changed response
	// when the volume moves. Without the label the phone is never told.
	watching byte
	watched  bool

	label byte
}

// State is everything worth putting on a screen.
type State struct {
	Track    Track
	Progress Progress

	// UID is which entry of the now playing list is the one playing, for a phone that says. All
	// ones means nothing is, and a phone that does not keep a list sends zero.
	UID uint64

	// Player is which player the far end answers for, for the lists that are asked of it by number.
	Player uint16

	// Volume is what the phone last set, as a percentage, or -1 if it has never said.
	Volume int
}

// New is a controller that has not spoken to anything yet.
func New() *Controller {
	return &Controller{
		pending:     map[byte]byte{},
		arting:      map[byte]uint32{},
		registering: map[byte]byte{},
		registered:  map[byte]bool{},
		supported:   map[byte]bool{},
		state:       State{Volume: -1},
	}
}

// State is what is currently known.
func (c *Controller) State() State { return c.state }

// nextLabel hands out a transaction label, skipping any still waiting on an answer.
//
// Reusing one that is outstanding means the next response cannot be matched to its question, and
// what arrives is a play status parsed as a track listing.
func (c *Controller) nextLabel() (byte, error) {
	for range 16 {
		c.label = (c.label + 1) & 0x0f
		if _, busy := c.pending[c.label]; !busy {
			return c.label, nil
		}
	}

	held := make(map[byte]string, len(c.pending))
	for label, pdu := range c.pending {
		if event, ok := c.registering[label]; ok {
			held[label] = fmt.Sprintf("notify %#02x", event)
			continue
		}
		held[label] = fmt.Sprintf("pdu %#02x", pdu)
	}
	return 0, fmt.Errorf("avrcp: all sixteen transaction labels are outstanding: %v", held)
}

// command wraps a frame as a transport message and remembers what was asked.
func (c *Controller) command(frame AVC, expecting byte) (Transport, error) {
	label, err := c.nextLabel()
	if err != nil {
		return Transport{}, err
	}

	c.pending[label] = expecting
	return Transport{Label: label, Type: MessageCommand, Payload: frame.Marshal()}, nil
}

// Press is a button, as the two messages that make one.
//
// Both halves, always. A press with no release leaves the phone holding the button down, which on
// a seek runs to the end of the track and on play does nothing visible until something else is
// pressed.
func (c *Controller) Press(op byte) ([]Transport, error) {
	down, err := c.command(PassThrough(op, Pressed), OpPassThrough)
	if err != nil {
		return nil, err
	}

	up, err := c.command(PassThrough(op, Released), OpPassThrough)
	if err != nil {
		return nil, err
	}

	return []Transport{down, up}, nil
}

// Start is what to send when the channel comes up: ask what the phone can notify about, then ask
// what is playing.
//
// Capabilities first. Registering for an event a phone does not support is refused once per event
// and per registration, which on a phone that renews on a timer is a steady trickle of rejections.
func (c *Controller) Start() ([]Transport, error) {
	capabilities, err := c.command(GetCapabilities(CapabilityEvents).Frame(Status), PDUGetCapabilities)
	if err != nil {
		return nil, err
	}

	attributes, err := c.command(GetElementAttributes(Basics...).Frame(Status), PDUGetElementAttributes)
	if err != nil {
		return nil, err
	}

	cover, err := c.cover()
	if err != nil {
		return nil, err
	}

	status, err := c.command(GetPlayStatus().Frame(Status), PDUGetPlayStatus)
	if err != nil {
		return nil, err
	}

	return []Transport{capabilities, attributes, cover, status}, nil
}

// Refresh asks again what is playing and where it has got to, for a caller that wants to poll
// rather than rely on notifications.
func (c *Controller) Refresh() ([]Transport, error) {
	attributes, err := c.command(GetElementAttributes(Basics...).Frame(Status), PDUGetElementAttributes)
	if err != nil {
		return nil, err
	}

	cover, err := c.cover()
	if err != nil {
		return nil, err
	}

	status, err := c.command(GetPlayStatus().Frame(Status), PDUGetPlayStatus)
	if err != nil {
		return nil, err
	}

	return []Transport{attributes, cover, status}, nil
}

// Attributes asks about the current track, naming which attributes are wanted.
//
// Refresh asks for everything, which is what a caller normally wants. This is for the ones a target
// leaves out of everything and hands over only when named.
func (c *Controller) Attributes(ids ...uint32) ([]Transport, error) {
	t, err := c.command(GetElementAttributes(ids...).Frame(Status), PDUGetElementAttributes)
	if err != nil {
		return nil, err
	}
	return []Transport{t}, nil
}

// Receive takes one message off the channel and returns what to send back.
func (c *Controller) Receive(t Transport) ([]Transport, error) {
	if t.Invalid {
		delete(c.pending, t.Label)
		return nil, fmt.Errorf("avrcp: the far end does not speak this profile")
	}

	frame, err := ParseAVC(t.Payload)
	if err != nil {
		return nil, err
	}

	if t.Type == MessageCommand {
		return c.commanded(t, frame)
	}
	return c.answered(t, frame)
}

// commanded answers a command the phone sent us.
//
// The only one worth answering is absolute volume: the phone's own slider moving this device's
// output, which is what a phone expects of anything it is playing to. Everything else is refused
// rather than ignored, because a phone waiting on an answer retries until it gives up on the
// channel entirely.
func (c *Controller) commanded(t Transport, frame AVC) ([]Transport, error) {
	answer := func(payload AVC) []Transport {
		return []Transport{{
			Label:   t.Label,
			Type:    MessageResponse,
			Payload: payload.Marshal(),
		}}
	}

	if frame.Opcode != OpVendorDependent {
		frame.Code = NotImplemented
		return answer(frame), nil
	}

	p, err := ParsePDU(frame.Operands)
	if err != nil {
		return nil, err
	}

	switch p.ID {
	case PDUSetAbsoluteVolume:
		if len(p.Params) < 1 {
			return nil, ErrShort
		}

		v := p.Params[0] & 0x7f
		c.state.Volume = Percent(v)
		if c.Volume != nil {
			c.Volume(c.state.Volume)
		}
		c.publish()

		// Answered with the volume actually set, which may not be the one asked for.
		return answer(SetAbsoluteVolume(v).Frame(Accepted)), nil

	case PDURegisterNotification:
		if len(p.Params) < 1 {
			return nil, ErrShort
		}

		// The phone registering for our volume. Interim carries what it is now; the change that
		// follows is sent by whoever moves the volume, not from here.
		if p.Params[0] == EventVolume {
			c.watching, c.watched = t.Label, true

			v := byte(0)
			if c.state.Volume >= 0 {
				v = Volume(c.state.Volume)
			}
			body := PDU{ID: PDURegisterNotification, Params: []byte{EventVolume, v}}
			return answer(body.Frame(Interim)), nil
		}
	}

	frame.Code = NotImplemented
	return answer(frame), nil
}

// answered takes a response to something we asked.
func (c *Controller) answered(t Transport, frame AVC) ([]Transport, error) {
	asked, waiting := c.pending[t.Label]
	if !waiting {
		// An answer to a question nobody asked. Not an error — a label may have been given up on
		// and the answer arrived late — but there is nothing to do with it.
		return nil, nil
	}

	// Interim is not the end of a transaction: the real answer follows on the same label. Every
	// other response closes it.
	if frame.Code != Interim {
		delete(c.pending, t.Label)
	}

	if frame.Code == Rejected || frame.Code == NotImplemented {
		return c.refused(t.Label, asked, frame)
	}

	if frame.Opcode == OpPassThrough {
		return nil, nil
	}
	if frame.Opcode != OpVendorDependent {
		return nil, nil
	}

	p, err := ParsePDU(frame.Operands)
	if err != nil {
		return nil, err
	}

	// A response too big for one packet. Nothing here reassembles, so the far end is told to stop
	// rather than left holding the rest of it open.
	if p.Packet == PacketStart || p.Packet == PacketContinue {
		return c.command1(AbortContinuing(p.ID).Frame(Control), PDUAbortContinuing)
	}

	switch p.ID {
	case PDUGetCapabilities:
		return c.capabilities(p)
	case PDUGetElementAttributes:
		return nil, c.attributes(t.Label, p)
	case PDUGetPlayStatus:
		return nil, c.playStatus(p)
	case PDURegisterNotification:
		return c.notified(t.Label, p, frame.Code == Interim)
	}
	return nil, nil
}

// refused is a command the phone would not do.
//
// A registration that is refused is dropped rather than retried, since retrying is how a phone that
// does not report positions gets asked about them forever. The one exception is the addressed
// player moving, which voids a registration without refusing it.
//
// Which event this was is the label's to say. A rejection carries an error code as its only
// parameter, where a completion would carry the event, so reading the parameter as an event marks
// some unrelated number unsupported and leaves the real one looking registered forever.
func (c *Controller) refused(label, asked byte, frame AVC) ([]Transport, error) {
	if asked != PDURegisterNotification {
		return nil, fmt.Errorf("avrcp: %#02x was refused", asked)
	}

	event, known := c.registering[label]
	delete(c.registering, label)
	if !known {
		return nil, nil
	}
	c.registered[event] = false

	status := byte(RefusedBadCommand)
	if p, err := ParsePDU(frame.Operands); err == nil && len(p.Params) > 0 {
		status = p.Params[0]
	}

	if status == RefusedPlayerMoved {
		return c.register(event)
	}

	c.supported[event] = false
	slog.Debug("the far end will not report an event",
		"event", fmt.Sprintf("%#02x", event), "status", fmt.Sprintf("%#02x", status))
	return nil, nil
}

// capabilities records what the phone will notify about, then registers for the ones worth having.
func (c *Controller) capabilities(p PDU) ([]Transport, error) {
	events, err := ParseCapabilities(p.Params)
	if err != nil {
		return nil, err
	}

	c.asked = true
	for _, e := range events {
		c.supported[e] = true
	}

	slog.Debug("the far end reports these events", "events", fmt.Sprintf("% #02x", events))

	// The last two are the multi-player half and a phone offers them only when it believes this end
	// speaks that much of the profile. They are what says the metadata now belongs to something
	// else, which nothing else on the channel reports.
	return c.register(EventPlaybackStatus, EventTrackChanged, EventPosition,
		EventNowPlaying, EventAddressPlayer)
}

// register asks to be told about the events the phone said it can report.
//
// Position gets an interval of one second. It is the only event where the interval means anything,
// and asking for a shorter one buys a redraw the eye cannot see at the cost of a round trip.
func (c *Controller) register(events ...byte) ([]Transport, error) {
	var out []Transport

	for _, e := range events {
		// Nothing is registered for twice: a second registration on the same event gets one more
		// answer, not a faster one.
		if c.registered[e] {
			continue
		}
		if c.asked && !c.supported[e] {
			continue
		}

		interval := uint32(0)
		if e == EventPosition {
			interval = 1
		}

		t, err := c.command(RegisterNotification(e, interval).Frame(Notify), PDURegisterNotification)
		if err != nil {
			return out, err
		}

		c.registered[e] = true
		c.registering[t.Label] = e
		out = append(out, t)
	}
	return out, nil
}

// cover asks for the artwork on its own, remembering which track it was asked about.
func (c *Controller) cover() (Transport, error) {
	t, err := c.command(GetElementAttributes(AttrCoverArt).Frame(Status), PDUGetElementAttributes)
	if err != nil {
		return t, err
	}

	c.arting[t.Label] = c.age
	return t, nil
}

func (c *Controller) attributes(label byte, p PDU) error {
	track, err := ParseElementAttributes(p.Params)
	if err != nil {
		return err
	}

	// The artwork is asked for on its own, so an answer carrying nothing else is that one. One
	// asked about a track that has since been replaced belongs to that track and not this one.
	if track.Empty() {
		age, asked := c.arting[label]
		delete(c.arting, label)

		if asked && age == c.age && track.Art != "" && track.Art != c.state.Track.Art {
			c.state.Track.Art = track.Art
			c.publish()
		}
		return nil
	}
	delete(c.arting, label)

	// The artwork is asked for on its own, so an answer about the rest carries none. The same track
	// keeps the handle it already has.
	if track.Art == "" && track.Title == c.state.Track.Title &&
		track.Artist == c.state.Track.Artist && track.Album == c.state.Track.Album {
		track.Art = c.state.Track.Art
	}

	if track == c.state.Track {
		return nil
	}

	if track.Title != c.state.Track.Title || track.Artist != c.state.Track.Artist {
		c.age++
	}

	c.state.Track = track
	c.publish()
	return nil
}

func (c *Controller) playStatus(p PDU) error {
	progress, err := ParsePlayStatus(p.Params)
	if err != nil {
		return err
	}

	c.state.Progress = progress
	c.publish()
	return nil
}

// notified handles a registration's answer, and renews it.
//
// A registration is one-shot. The interim answer carries the current value and the one after it
// carries the change — and once that has arrived the phone has forgotten the registration, so it
// has to be made again or the screen updates exactly once.
func (c *Controller) notified(label byte, p PDU, interim bool) ([]Transport, error) {
	n, err := ParseNotification(p.Params, interim)
	if err != nil {
		return nil, err
	}

	// Interim leaves the transaction open, so the label still says what it is for.
	if !interim {
		delete(c.registering, label)
	}

	was := c.state.Progress.Status
	moved := false

	switch n.Event {
	case EventPlaybackStatus:
		c.state.Progress.Status = n.Status
	case EventPosition:
		c.state.Progress.Position = n.Position
	case EventVolume:
		c.state.Volume = Percent(n.Volume)
	case EventTrackChanged:
		c.state.UID = n.Track

	case EventAddressPlayer:
		// The same player again is the interim answer restating where things stand, which is not a
		// move and not worth a round trip.
		moved = n.Player != c.player
		c.player = n.Player
		c.state.Player = n.Player

		if moved {
			slog.Debug("the far end addressed a different player", "player", n.Player)
		}

	case EventNowPlaying:
		moved = !interim
	}
	c.publish()

	// Playing when it was not, however it was said. An interim carries the value as it stands, so a
	// far end that has already started by the time it answers reports the start there and never
	// sends a change at all.
	started := n.Event == EventPlaybackStatus && n.Status == StatusPlaying && was != StatusPlaying

	var out []Transport
	if !interim {
		// Spent, so renew it or the screen updates exactly once.
		c.registered[n.Event] = false

		out, err = c.register(n.Event)
		if err != nil {
			return out, err
		}
	}

	// A track change says only that it changed, so ask what to. Starting is asked about too, since
	// a source that reports the status and not the track gives no other sign — and so is the
	// addressed player moving, where what is on the screen belongs to whatever held it before.
	if n.Event == EventTrackChanged || started || moved {
		more, err := c.Refresh()
		if err != nil {
			return out, err
		}
		out = append(out, more...)
	}
	return out, nil
}

// command1 is one command, as the single message that carries it.
func (c *Controller) command1(frame AVC, expecting byte) ([]Transport, error) {
	t, err := c.command(frame, expecting)
	if err != nil {
		return nil, err
	}
	return []Transport{t}, nil
}

// Volumed answers the phone's outstanding volume registration, for a volume moved at this end. One
// answer per registration; the phone registers again after each.
//
// Nothing is sent for a volume the phone just set, or the two ends chase each other up the scale.
func (c *Controller) Volumed(percent int) []Transport {
	if percent == c.state.Volume {
		return nil
	}
	c.state.Volume = percent
	c.publish()

	if !c.watched {
		return nil
	}
	c.watched = false

	body := PDU{ID: PDURegisterNotification, Params: []byte{EventVolume, Volume(percent)}}
	return []Transport{{
		Label:   c.watching,
		Type:    MessageResponse,
		Payload: body.Frame(Changed).Marshal(),
	}}
}

// SetVolume tells the phone how loud this device now is, for a volume changed at this end.
func (c *Controller) SetVolume(percent int) (Transport, error) {
	c.state.Volume = percent
	return c.command(SetAbsoluteVolume(Volume(percent)).Frame(Control), PDUSetAbsoluteVolume)
}

// Forget drops everything known, for a phone that has gone away. The next one to connect starts
// from nothing rather than inheriting the last one's track.
func (c *Controller) Forget() {
	clear(c.pending)
	clear(c.registering)
	clear(c.registered)
	clear(c.supported)
	c.player = 0
	c.asked = false
	c.state = State{Volume: -1}
}

// Cleared forgets what was playing, for a stream that has been replaced.
//
// A blank answer within a stream is kept, since a phone between tracks sends one. Across a stream
// it is not: an app that says nothing leaves the old title over the wrong song.
//
// The volume and the registrations belong to the link rather than the track, and stay.
func (c *Controller) Cleared() {
	c.state.Track = Track{}
	c.state.Progress = Progress{}
	c.state.UID = 0
	c.publish()
}

func (c *Controller) publish() {
	if c.Changed != nil {
		c.Changed(c.state)
	}
}

// Elapsed is how far through the track the far end is, for a progress bar that has to keep moving
// between notifications.
//
// Position arrives once a second at best and only while something is registered for it, so a bar
// drawn from it alone steps rather than sweeps. This carries it forward by how long ago it was
// heard, and only while something is actually playing.
func (s State) Elapsed(since time.Duration) time.Duration {
	if s.Progress.Status != StatusPlaying {
		return s.Progress.Position
	}

	at := s.Progress.Position + since
	if s.Progress.Length > 0 && at > s.Progress.Length {
		return s.Progress.Length
	}
	return at
}
