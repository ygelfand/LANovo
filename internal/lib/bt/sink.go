package bt

import (
	"fmt"
	"slices"

	"github.com/ygelfand/LANovo/internal/lib/bt/avdtp"
	"github.com/ygelfand/LANovo/internal/lib/bt/avrcp"
	"github.com/ygelfand/LANovo/internal/lib/bt/l2cap"
	"github.com/ygelfand/LANovo/internal/lib/bt/sbc"
	"github.com/ygelfand/LANovo/internal/lib/bt/sdp"
)

// Sink is the whole stack above ACL: the thing a phone connects to and plays music at.
//
// Frames arrive from the radio and go in one end; frames to send come out the other, and audio
// comes out sideways through Frame. Nothing in here touches hardware, which is the point — the
// device this runs on cannot run a test, so the part that can be tested is kept free of it.
//
// A2DP opens three channels and the difference between them is not in anything they carry:
//
//	SDP (0x0001)    what this device is, opened and usually closed again
//	AVDTP (0x0019)  signalling — the first channel to that number
//	AVDTP (0x0019)  media — the second channel to that same number
//
// The two AVDTP channels are told apart by which came first and by nothing else. A phone opens
// signalling, negotiates a configuration on it, and then opens the second for the audio.
//
// Not safe for concurrent use: it is driven from whichever goroutine reads the radio.
type Sink struct {
	l2      *l2cap.Manager
	stream  *avdtp.Server
	remote  *avrcp.Controller
	records []sdp.Record

	// The two AVDTP channels, by local number, once each has opened. Zero is not a valid channel
	// number, so it doubles as "not yet".
	signalling uint16
	transport  uint16

	// control is the AVCTP channel, which is its own connection to its own number.
	control uint16

	// browse is the second AVCTP channel, for the pdus about which player is addressed and what is
	// in its lists. This end opens that one.
	browse uint16

	// finding is a channel of ours to the far end's service records, for looking up where it serves
	// something. Separate from the one a phone opens to read ours, which arrives on the same
	// protocol number and is a different channel.
	finding uint16

	// transaction numbers our own lookups, so an answer can be told from a late one.
	transaction uint16

	// art is the channel the far end serves images on, whose number it chose and published.
	art    uint16
	artPSM uint16

	media   avdtp.Reassembler
	command avrcp.Reassembler

	// Frame is called with each complete SBC frame, in order. Whatever decodes audio hangs here.
	// Nil is fine; a sink that only negotiates is a legitimate thing to be while the rest is built.
	Frame func([]byte)

	// Started and Stopped bracket the audio, which is when a decoder wants configuring and tearing
	// down. The endpoint carries the configuration the phone chose.
	Started func(*avdtp.Endpoint)
	Stopped func(*avdtp.Endpoint)

	// Controls is called when the remote control channel opens, for a caller that wants to ask the
	// phone something. Frames cannot be returned from here — this happens while a signalling frame
	// is being answered — so a caller sends them itself.
	Controls func()

	// Opened is called when a channel this end asked for comes up, named by the protocol it was
	// asked on. Nothing can be sent over one until it has.
	Opened func(psm uint16)

	// Listed is called with each answer that arrives on the browsing channel, for a caller that
	// asked something of it.
	Listed func(avrcp.Browse)

	// Found is called with the records a lookup of the far end returned, and whether it held back
	// more than fitted.
	Found func(records []sdp.Record, more bool)

	// Image is called with each packet that arrives on the image channel, still encoded. What it
	// means depends on what was asked, which only the caller knows.
	Image func(packet []byte)
}

// SEID is the stream endpoint this device offers. Any number from 1 to 0x3e would do; a phone
// learns it by asking.
const SEID = 1

// MTU is what this end accepts on a channel.
//
// Larger than the classic default, because a browsing answer is packed until the next entry does
// not fit and then stopped: at 672 a listing comes back as four or five tracks whatever the phone
// says it holds, and fewer than that when the titles are long.
const MTU = 2048

// NewSink is a sink answering to a name, serving SDP, AVDTP and AVCTP.
func NewSink(name string, mtu uint16) *Sink {
	s := &Sink{
		l2:      l2cap.NewManager(mtu),
		stream:  avdtp.NewServer(SEID),
		remote:  avrcp.New(),
		records: Records(name),
	}

	s.l2.Serve(l2cap.PSMSDP)
	s.l2.Serve(SinkPSM)
	s.l2.Serve(ControlPSM)
	s.l2.Serve(BrowsePSM)

	s.stream.Started = func(e *avdtp.Endpoint) {
		if s.Started != nil {
			s.Started(e)
		}
	}
	s.stream.Stopped = func(e *avdtp.Endpoint) {
		s.media = avdtp.Reassembler{}
		if s.Stopped != nil {
			s.Stopped(e)
		}
	}

	return s
}

// Endpoint is the stream endpoint and whatever has been agreed on it.
func (s *Sink) Endpoint() *avdtp.Endpoint { return s.stream.Endpoint(SEID) }

// Receive takes one L2CAP frame off the link and returns what to send back.
//
// An error here is a malformed frame, not a broken link: the caller logs it and reads the next one.
// Dropping the connection because one packet did not parse would turn a glitch into a device that
// stops playing music.
func (s *Sink) Receive(f l2cap.Frame) ([]l2cap.Frame, error) {
	if f.CID == l2cap.CIDSignalling {
		commands, err := s.l2.Handle(f)
		s.adopt()

		if len(commands) == 0 {
			return nil, err
		}
		return []l2cap.Frame{l2cap.Signal(commands...)}, err
	}

	ch := s.l2.Channel(f.CID)
	if ch == nil {
		return nil, fmt.Errorf("bt: data on channel %#x, which is not open", f.CID)
	}

	switch {
	case ch.PSM == l2cap.PSMSDP:
		return s.discovery(ch, f.Payload)
	case ch.PSM == ControlPSM:
		return s.avctp(ch, f.Payload)
	case ch.PSM == BrowsePSM:
		return s.browsing(ch, f.Payload)
	case f.CID == s.signalling:
		return s.signal(ch, f.Payload)
	case f.CID == s.transport:
		return nil, s.audio(f.Payload)

	// By number rather than by protocol: the channel images come over is allocated when the far
	// end's service starts, so there is no fixed number to match against.
	case f.CID == s.art:
		if s.Image != nil {
			s.Image(f.Payload)
		}
		return nil, nil
	}

	return nil, fmt.Errorf("bt: channel %#x carries nothing this understands", f.CID)
}

// adopt works out which channel is which as they open.
//
// AVCTP is unambiguous: one channel to that number and it is the remote control. AVDTP is not —
// signalling and media are two connections to the same number, and order is the only thing that
// distinguishes them. A phone that opened them the other way round would be one that does not
// speak AVDTP.
func (s *Sink) adopt() {
	var control, browse, finding, art uint16
	var stream []*l2cap.Channel

	for _, c := range s.l2.Open() {
		switch {
		case c.PSM == ControlPSM:
			control = c.Local
		case c.PSM == BrowsePSM:
			browse = c.Local

		// Only the one this end opened. A phone reading our records arrives on the same protocol
		// number, and answers to our questions do not come back on its channel.
		case c.PSM == l2cap.PSMSDP && c.Ours:
			finding = c.Local

		case s.artPSM != 0 && c.PSM == s.artPSM:
			art = c.Local
		case c.PSM == SinkPSM:
			stream = append(stream, c)
		}
	}

	// Lowest number first: the manager hands them out in the order they are asked for, so the
	// signalling channel is the one with the smaller number.
	slices.SortFunc(stream, func(a, b *l2cap.Channel) int { return int(a.Local) - int(b.Local) })

	s.signalling, s.transport = 0, 0
	if len(stream) > 0 {
		s.signalling = stream[0].Local
	}
	if len(stream) > 1 {
		s.transport = stream[1].Local
	}

	// The control channel going or changing is the session ending. Whatever this end opened
	// belonged to it, and the far end will not close a channel it did not open, so those go here or
	// they never go at all.
	if was := s.control; was != control {
		s.control = control
		s.stale()
		browse, finding, art = 0, 0, 0

		if control != 0 && s.Controls != nil {
			s.Controls()
		}
	}

	s.came(&s.browse, browse, BrowsePSM)
	s.came(&s.finding, finding, l2cap.PSMSDP)
	s.came(&s.art, art, s.artPSM)
}

// stale drops the channels this end opened.
func (s *Sink) stale() {
	for _, at := range []*uint16{&s.browse, &s.finding, &s.art} {
		if *at != 0 {
			s.l2.Drop(*at)
			*at = 0
		}
	}
	s.artPSM = 0
}

// took records where a channel is now and reports a number that was not there before.
func took(at *uint16, now uint16) bool {
	was := *at
	*at = now
	return now != 0 && now != was
}

// came is took for the channels this end opens, which say so through one hook.
func (s *Sink) came(at *uint16, now, psm uint16) {
	if took(at, now) && s.Opened != nil {
		s.Opened(psm)
	}
}

// Ask asks the phone what it is playing and to say when it changes. Nothing arrives unprompted.
func (s *Sink) Ask() ([]l2cap.Frame, error) {
	messages, err := s.remote.Start()
	if err != nil {
		return nil, err
	}
	return s.Control(messages)
}

// Media reports whether a channel is the one audio arrives on, for a caller deciding whether a
// packet is worth saying anything about. Everything else on a link is a handful of messages; this
// one is hundreds a second.
func (s *Sink) Media(cid uint16) bool { return cid != 0 && cid == s.transport }

// Remote is the remote control: what is playing at the far end, and the buttons that drive it.
func (s *Sink) Remote() *avrcp.Controller { return s.remote }

// Controlling reports whether the remote control channel is up, which is what says the buttons
// will reach anything.
func (s *Sink) Controlling() bool { return s.control != 0 }

// avctp answers one remote control message.
func (s *Sink) avctp(ch *l2cap.Channel, payload []byte) ([]l2cap.Frame, error) {
	// A metadata response with a long title arrives in pieces, so nothing is parsed until the
	// whole message is in hand.
	whole, err := s.command.Push(payload)
	if err != nil {
		return nil, err
	}
	if whole == nil {
		return nil, nil
	}

	m, err := avrcp.ParseTransport(whole)
	if err != nil {
		return nil, err
	}

	answers, err := s.remote.Receive(m)
	if err != nil {
		return nil, err
	}

	var out []l2cap.Frame
	for _, a := range answers {
		out = append(out, s.send(ch, a.Marshal())...)
	}
	return out, nil
}

// browsing answers one message on the browsing channel.
//
// No reassembler: the browsing channel does not fragment at AVCTP, and a pdu too large for the
// channel is the sender's problem rather than something to piece together here.
func (s *Sink) browsing(ch *l2cap.Channel, payload []byte) ([]l2cap.Frame, error) {
	m, err := avrcp.ParseTransport(payload)
	if err != nil {
		return nil, err
	}

	// An answer to something this end asked. Browsed only speaks for the responder half, so a
	// response would otherwise fall through it silently.
	if m.Type == avrcp.MessageResponse {
		if s.Listed == nil {
			return nil, nil
		}

		b, err := avrcp.ParseBrowse(m.Payload)
		if err != nil {
			return nil, err
		}
		s.Listed(b)
		return nil, nil
	}

	answers, bad := avrcp.Browsed(m)

	var out []l2cap.Frame
	for _, a := range answers {
		out = append(out, s.send(ch, a.Marshal())...)
	}
	return out, bad
}

// Control sends AVCTP messages the controller produced, for a caller that pressed a button.
//
// Nothing comes back: a button's answer arrives later as an ordinary message on the channel.
func (s *Sink) Control(messages []avrcp.Transport) ([]l2cap.Frame, error) {
	if s.control == 0 {
		return nil, fmt.Errorf("bt: the remote control channel is not open")
	}

	ch := s.l2.Channel(s.control)
	if ch == nil {
		return nil, fmt.Errorf("bt: the remote control channel went away")
	}

	var out []l2cap.Frame
	for _, m := range messages {
		out = append(out, s.send(ch, m.Marshal())...)
	}
	return out, nil
}

// Volumed is this device's volume having moved, as the frames that tell the phone.
//
// Empty when the phone never asked to be told, or when the volume it is being told about is the one
// it just set. Neither is a fault, so neither is an error.
func (s *Sink) Volumed(percent int) ([]l2cap.Frame, error) {
	messages := s.remote.Volumed(percent)
	if len(messages) == 0 {
		return nil, nil
	}
	return s.Control(messages)
}

// Browsing reports whether the browsing channel is up and usable.
func (s *Sink) Browsing() bool {
	if s.browse == 0 {
		return false
	}
	ch := s.l2.Channel(s.browse)
	return ch != nil && ch.State == l2cap.Open
}

// OpenBrowsing asks the phone for the browsing channel, as the frame that carries the request.
//
// This one is ours to open: the control channel arrives with the phone, and the pdus about which
// player is addressed and what is in its lists only travel on this one. Nothing is usable until the
// far end has answered and both ends have configured, so a caller waits for Browsing.
func (s *Sink) OpenBrowsing() ([]l2cap.Frame, error) {
	if s.browse != 0 {
		return nil, fmt.Errorf("bt: the browsing channel is already open")
	}

	c, err := s.l2.Connect(BrowsePSM)
	if err != nil {
		return nil, err
	}
	return []l2cap.Frame{l2cap.Signal(c)}, nil
}

// Browse sends AVCTP messages on the browsing channel.
func (s *Sink) Browse(messages []avrcp.Transport) ([]l2cap.Frame, error) {
	if !s.Browsing() {
		return nil, fmt.Errorf("bt: the browsing channel is not open")
	}

	ch := s.l2.Channel(s.browse)

	var out []l2cap.Frame
	for _, m := range messages {
		out = append(out, s.send(ch, m.Marshal())...)
	}
	return out, nil
}

// OpenArt asks for the channel the far end serves images on, as the frame that carries the request.
//
// The number is not fixed: the far end allocates it when its service starts and publishes it in its
// own record, so a caller looks it up rather than assuming one.
func (s *Sink) OpenArt(psm uint16) ([]l2cap.Frame, error) {
	if psm == 0 {
		return nil, fmt.Errorf("bt: no channel was published for images")
	}
	if s.art != 0 {
		return nil, fmt.Errorf("bt: the image channel is already open")
	}

	c, err := s.l2.Connect(psm)
	if err != nil {
		return nil, err
	}

	s.artPSM = psm
	return []l2cap.Frame{l2cap.Signal(c)}, nil
}

// Imaging reports whether the image channel is up.
func (s *Sink) Imaging() bool {
	if s.art == 0 {
		return false
	}
	ch := s.l2.Channel(s.art)
	return ch != nil && ch.State == l2cap.Open
}

// Art sends one already encoded packet on the image channel.
func (s *Sink) Art(packet []byte) ([]l2cap.Frame, error) {
	if s.art == 0 {
		return nil, fmt.Errorf("bt: the image channel is not open")
	}

	ch := s.l2.Channel(s.art)
	if ch == nil || ch.State != l2cap.Open {
		return nil, fmt.Errorf("bt: the image channel is not open")
	}
	return s.send(ch, packet), nil
}

// Attributes asks about the current track, naming which attributes are wanted.
func (s *Sink) Attributes(ids ...uint32) ([]l2cap.Frame, error) {
	messages, err := s.remote.Attributes(ids...)
	if err != nil {
		return nil, err
	}
	return s.Control(messages)
}

// Press is a button on the screen, as the frames that carry it to the phone.
func (s *Sink) Press(op byte) ([]l2cap.Frame, error) {
	messages, err := s.remote.Press(op)
	if err != nil {
		return nil, err
	}
	return s.Control(messages)
}

// discovery answers one SDP request.
func (s *Sink) discovery(ch *l2cap.Channel, payload []byte) ([]l2cap.Frame, error) {
	p, err := sdp.ParsePDU(payload)
	if err != nil {
		return nil, err
	}

	// Both directions run on the same protocol number, so which this is comes from the message
	// rather than the channel: an answer is one we asked for.
	switch p.ID {
	case sdp.PDUSearchAttributeResponse:
		return nil, s.found(p)

	case sdp.PDUError:
		return nil, fmt.Errorf("sdp: the far end refused a search: % x", p.Params)
	}

	answer, err := s.answer(p)
	if err != nil {
		return nil, err
	}
	return s.send(ch, answer.Marshal()), nil
}

// found hands over the records an answer carried.
//
// A continuation is not followed. Everything worth asking this far has fitted, and chasing one
// without knowing the far end will stop is a loop rather than a fetch.
func (s *Sink) found(p sdp.PDU) error {
	records, more, err := sdp.ParseSearchAttributeResponse(p.Params)
	if err != nil {
		return err
	}
	if s.Found != nil {
		s.Found(records, len(more) > 0)
	}
	return nil
}

// OpenDiscovery asks for a channel to the far end's service records, as the frame that carries the
// request. Nothing about the far end is known until it answers.
func (s *Sink) OpenDiscovery() ([]l2cap.Frame, error) {
	if s.finding != 0 {
		return nil, fmt.Errorf("bt: already looking the far end up")
	}

	c, err := s.l2.Connect(l2cap.PSMSDP)
	if err != nil {
		return nil, err
	}
	return []l2cap.Frame{l2cap.Signal(c)}, nil
}

// Finding reports whether there is a channel to ask the far end's records on.
func (s *Sink) Finding() bool {
	if s.finding == 0 {
		return false
	}
	ch := s.l2.Channel(s.finding)
	return ch != nil && ch.State == l2cap.Open
}

// Discover asks what the far end offers under a service class.
func (s *Sink) Discover(search sdp.Search) ([]l2cap.Frame, error) {
	if s.finding == 0 {
		return nil, fmt.Errorf("bt: no channel to look the far end up on")
	}

	ch := s.l2.Channel(s.finding)
	if ch == nil || ch.State != l2cap.Open {
		return nil, fmt.Errorf("bt: the lookup channel is not open")
	}

	s.transaction++
	p, err := search.Request(s.transaction)
	if err != nil {
		return nil, err
	}
	return s.send(ch, p.Marshal()), nil
}

// answer is the reply to one SDP request.
//
// Only ServiceSearchAttribute is implemented, which is what phones send. The other two get an
// error PDU rather than silence: a phone told no asks differently, a phone told nothing waits.
func (s *Sink) answer(p sdp.PDU) (sdp.PDU, error) {
	if p.ID != sdp.PDUSearchAttributeRequest {
		return sdp.Error(p.Transaction, sdp.ErrorBadSyntax), nil
	}

	search, err := sdp.ParseSearchAttribute(p.Params)
	if err != nil {
		return sdp.Error(p.Transaction, sdp.ErrorBadSyntax), nil
	}

	// Every record that matches, not the first. A phone searching the public browse group for
	// everything expects all three back in one answer, and sending one makes the other two look
	// absent.
	//
	// No match is an empty list, not an error. The phone asked a reasonable question and the
	// answer is that this device is not that.
	var records []sdp.Record
	for _, r := range s.records {
		if search.Wants(r.UUIDs()) {
			records = append(records, search.Selected(r))
		}
	}

	return sdp.SearchAttributeResponse(p.Transaction, records)
}

// signal answers one AVDTP message.
func (s *Sink) signal(ch *l2cap.Channel, payload []byte) ([]l2cap.Frame, error) {
	m, err := avdtp.ParseMessage(payload)
	if err != nil {
		return nil, err
	}

	answer := s.stream.Handle(m)

	// A response to something we did not send. Nothing here sends commands, so there is nothing to
	// reply to and nothing to say. No signal is zero, so an empty message is the whole of it.
	if answer.Signal == 0 {
		return nil, nil
	}
	return s.send(ch, answer.Marshal()), nil
}

// audio takes one media packet and hands on whatever frames are now complete.
func (s *Sink) audio(payload []byte) error {
	m, err := avdtp.ParseMedia(payload)
	if err != nil {
		return err
	}

	body := s.media.Push(m)
	if body == nil || s.Frame == nil {
		return nil
	}

	// A packet holds several frames back to back and says how many. Walking them by their own
	// lengths rather than trusting the count keeps a wrong count from reading into the next frame.
	for len(body) > 0 {
		n, ok := frameLength(body)
		if !ok {
			return fmt.Errorf("bt: %d bytes left over that are not a frame", len(body))
		}
		s.Frame(body[:n])
		body = body[n:]
	}
	return nil
}

// frameLength is how long the SBC frame at the front of a buffer is, and whether all of it is
// there. A frame carries its own length in its header, which is what lets a run of them be walked
// without being told how many there are.
func frameLength(buf []byte) (int, bool) {
	h, err := sbc.ParseHeader(buf)
	if err != nil {
		return 0, false
	}

	n := h.Length()
	if n <= 0 || n > len(buf) {
		return 0, false
	}
	return n, true
}

// send splits a payload across as many frames as the channel's packet size needs.
func (s *Sink) send(ch *l2cap.Channel, payload []byte) []l2cap.Frame {
	mtu := int(ch.MTU)
	if mtu <= 0 {
		mtu = l2cap.MTUDefault
	}

	var out []l2cap.Frame
	for len(payload) > mtu {
		out = append(out, l2cap.Frame{CID: ch.Remote, Payload: payload[:mtu]})
		payload = payload[mtu:]
	}
	return append(out, l2cap.Frame{CID: ch.Remote, Payload: payload})
}

// Forget drops every channel and everything agreed on them, for a link that has gone away.
func (s *Sink) Forget() {
	s.l2.Forget()
	s.stream.Forget()
	s.remote.Forget()
	s.media = avdtp.Reassembler{}
	s.command = avrcp.Reassembler{}
	s.signalling = 0
	s.transport = 0
	s.control = 0
}
