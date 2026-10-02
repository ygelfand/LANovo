package avdtp

import "fmt"

// The state machine, and the endpoint it belongs to.
//
// An endpoint is idle until a phone configures it, open once a transport channel exists, and
// streaming once told to start. What makes this worth writing down rather than tracking loosely is
// that a command arriving in the wrong state has a specific refusal — BAD_STATE — and a stack that
// answers something else leaves the phone retrying a thing that will never work.

// State is where an endpoint is.
type State int

const (
	// Idle is configured by nobody. A phone may set a configuration on it.
	Idle State = iota

	// Configured has a codec agreed but no transport channel yet.
	Configured

	// Opened has a channel and is ready. Audio is not flowing.
	Opened

	// Streaming is audio flowing.
	Streaming
)

func (s State) String() string {
	switch s {
	case Configured:
		return "configured"
	case Opened:
		return "open"
	case Streaming:
		return "streaming"
	}
	return "idle"
}

// Endpoint is one stream endpoint and what has been agreed on it.
type Endpoint struct {
	SEP   SEP
	State State

	// Config is what the phone chose, once it has. Empty while idle.
	Config []Capability

	// Remote is the endpoint at the phone's end, which its configuration named. Answers about this
	// stream are addressed to it.
	Remote byte
}

// Codec is the SBC configuration agreed, for a caller that wants to set up a decoder.
func (e *Endpoint) Codec() (SBC, bool) {
	c, ok := Find(e.Config, CatMediaCodec)
	if !ok {
		return SBC{}, false
	}
	s, err := ParseSBC(c)
	if err != nil {
		return SBC{}, false
	}
	return s, true
}

// Server answers a phone's signalling for the endpoints this device offers.
//
// Not safe for concurrent use: it is driven from whichever goroutine reads the channel, and a lock
// would only hide that a second caller has no ordering to rely on.
type Server struct {
	endpoints []*Endpoint

	// Started and Stopped are called as audio begins and ends, for whatever is going to decode it.
	// Nil is fine; a sink that only wants to negotiate is a legitimate thing to be.
	Started func(*Endpoint)
	Stopped func(*Endpoint)
}

// NewServer offers one audio sink, which is what A2DP needs and all this advertises.
func NewServer(seid byte) *Server {
	return &Server{
		endpoints: []*Endpoint{{
			SEP: SEP{SEID: seid, Media: MediaAudio, TSEP: Sink},
		}},
	}
}

// Endpoint is the one with this identifier, or nil.
func (s *Server) Endpoint(seid byte) *Endpoint {
	for _, e := range s.endpoints {
		if e.SEP.SEID == seid {
			return e
		}
	}
	return nil
}

// Handle answers one message.
//
// Everything gets an answer. A phone waiting on a response it will never get holds the channel open
// and eventually drops the whole connection, which looks to somebody in the room like the speaker
// refusing to pair.
func (s *Server) Handle(m Message) Message {
	// Only commands are answered. A response arriving here is one to something we sent, and nothing
	// here sends commands — a sink is driven, it does not drive.
	if m.Type != Command {
		return Message{}
	}

	switch m.Signal {
	case SignalDiscover:
		return m.Accept(s.discover()...)

	case SignalGetCapabilities, SignalGetAllCaps:
		return s.capabilities(m)

	case SignalSetConfig:
		return s.setConfig(m)

	case SignalGetConfig:
		return s.getConfig(m)

	case SignalOpen:
		return s.transition(m, Configured, Opened, nil)

	case SignalStart:
		return s.startStop(m, Opened, Streaming, s.Started)

	case SignalSuspend:
		return s.startStop(m, Streaming, Opened, s.Stopped)

	case SignalClose:
		return s.close(m)

	case SignalAbort:
		return s.abort(m)
	}

	// A signal this does not implement. General reject is the answer for one that is not
	// recognised at all, which is different from one refused.
	return Message{Label: m.Label, Type: GeneralReject, Signal: m.Signal}
}

func (s *Server) discover() []byte {
	var out []byte
	for _, e := range s.endpoints {
		sep := e.SEP
		sep.InUse = e.State != Idle
		out = append(out, sep.Marshal()...)
	}
	return out
}

func (s *Server) capabilities(m Message) Message {
	seid, ok := SEID(m.Data)
	if !ok {
		return m.Reject(ErrorBadLength)
	}

	e := s.Endpoint(seid)
	if e == nil {
		return m.Reject(ErrorBadSEID)
	}
	return m.Accept(MarshalCapabilities(SinkCapabilities())...)
}

// setConfig is the one command that can fail for a reason worth naming, so it is the one that gets
// the category-carrying refusal.
func (s *Server) setConfig(m Message) Message {
	// Our endpoint, then theirs, then the capabilities they chose.
	if len(m.Data) < 2 {
		return m.Reject(ErrorBadLength)
	}

	e := s.Endpoint(m.Data[0] >> 2)
	if e == nil {
		return m.Reject(ErrorBadSEID)
	}
	if e.State != Idle {
		return m.Reject(ErrorSEPInUse)
	}

	chosen, err := ParseCapabilities(m.Data[2:])
	if err != nil {
		return m.RejectConfig(0, ErrorBadPayloadFormat)
	}

	// Media transport has to be there. Without it there is no agreement about how audio arrives,
	// whatever the codec says.
	if _, ok := Find(chosen, CatMediaTransport); !ok {
		return m.RejectConfig(CatMediaTransport, ErrorUnsupportedConfig)
	}

	codec, ok := Find(chosen, CatMediaCodec)
	if !ok {
		return m.RejectConfig(CatMediaCodec, ErrorUnsupportedConfig)
	}

	sbc, err := ParseSBC(codec)
	if err != nil {
		return m.RejectConfig(CatMediaCodec, ErrorUnsupportedConfig)
	}
	if !sbc.Chosen() {
		return m.RejectConfig(CatMediaCodec, ErrorBadPayloadFormat)
	}
	if !sbc.Within(SinkSBC()) {
		return m.RejectConfig(CatMediaCodec, ErrorUnsupportedConfig)
	}

	// Anything we were not asked about and do not implement is refused by naming it, which is what
	// lets the phone try again without it.
	for _, c := range chosen {
		switch c.Category {
		case CatMediaTransport, CatMediaCodec, CatDelayReporting:
		default:
			return m.RejectConfig(c.Category, ErrorBadCategory)
		}
	}

	e.Config = chosen
	e.Remote = m.Data[1] >> 2
	e.State = Configured

	return m.Accept()
}

func (s *Server) getConfig(m Message) Message {
	seid, ok := SEID(m.Data)
	if !ok {
		return m.Reject(ErrorBadLength)
	}

	e := s.Endpoint(seid)
	if e == nil {
		return m.Reject(ErrorBadSEID)
	}
	if e.State == Idle {
		return m.Reject(ErrorSEPNotInUse)
	}
	return m.Accept(MarshalCapabilities(e.Config)...)
}

// transition moves one endpoint between states, refusing when it is not where it should be.
func (s *Server) transition(m Message, from, to State, after func(*Endpoint)) Message {
	seid, ok := SEID(m.Data)
	if !ok {
		return m.Reject(ErrorBadLength)
	}

	e := s.Endpoint(seid)
	if e == nil {
		return m.Reject(ErrorBadSEID)
	}
	if e.State != from {
		return m.Reject(ErrorBadState)
	}

	e.State = to
	if after != nil {
		after(e)
	}
	return m.Accept()
}

// startStop is Start and Suspend, which differ from the rest by naming several endpoints at once.
//
// The refusal has to name which one failed, because the phone asked about a list and "no" without a
// subject tells it nothing about which to retry.
func (s *Server) startStop(m Message, from, to State, after func(*Endpoint)) Message {
	if len(m.Data) == 0 {
		return m.Reject(ErrorBadLength)
	}

	// Checked before anything moves: a list that is half applied leaves the phone and this device
	// disagreeing about which streams are running.
	for _, b := range m.Data {
		seid := b >> 2

		e := s.Endpoint(seid)
		if e == nil {
			return Message{Label: m.Label, Type: ResponseReject, Signal: m.Signal,
				Data: []byte{b, ErrorBadSEID}}
		}
		if e.State != from {
			return Message{Label: m.Label, Type: ResponseReject, Signal: m.Signal,
				Data: []byte{b, ErrorBadState}}
		}
	}

	for _, b := range m.Data {
		e := s.Endpoint(b >> 2)
		e.State = to
		if after != nil {
			after(e)
		}
	}
	return m.Accept()
}

func (s *Server) close(m Message) Message {
	seid, ok := SEID(m.Data)
	if !ok {
		return m.Reject(ErrorBadLength)
	}

	e := s.Endpoint(seid)
	if e == nil {
		return m.Reject(ErrorBadSEID)
	}
	if e.State == Idle {
		return m.Reject(ErrorBadState)
	}

	streaming := e.State == Streaming
	s.release(e)

	if streaming && s.Stopped != nil {
		s.Stopped(e)
	}
	return m.Accept()
}

// abort takes an endpoint back to idle from wherever it was, and is never refused for state.
//
// That is the point of it: a phone aborts when it has lost track of where things are, and an abort
// that can fail leaves both ends stuck.
func (s *Server) abort(m Message) Message {
	seid, ok := SEID(m.Data)
	if !ok {
		return m.Reject(ErrorBadLength)
	}

	e := s.Endpoint(seid)
	if e == nil {
		return m.Reject(ErrorBadSEID)
	}

	streaming := e.State == Streaming
	s.release(e)

	if streaming && s.Stopped != nil {
		s.Stopped(e)
	}
	return m.Accept()
}

func (s *Server) release(e *Endpoint) {
	e.State = Idle
	e.Config = nil
	e.Remote = 0
}

// Forget puts every endpoint back to idle, for a link that has gone away.
func (s *Server) Forget() {
	for _, e := range s.endpoints {
		if e.State == Streaming && s.Stopped != nil {
			s.Stopped(e)
		}
		s.release(e)
	}
}

// String is the server's state, for a log line that has to say what a stream is doing.
func (s *Server) String() string {
	if len(s.endpoints) == 0 {
		return "no endpoints"
	}
	e := s.endpoints[0]
	return fmt.Sprintf("endpoint %d %v", e.SEP.SEID, e.State)
}
