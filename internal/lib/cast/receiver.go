package cast

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// The receiver: what a sender is talking to, and what it does about each thing it is told.
//
// Messages in, messages out. Nothing here owns a socket — the caller reads frames off one
// connection and hands them over, and writes back whatever comes out. That is what lets the whole
// conversation be driven by a test.
//
// Not safe for concurrent use: it is driven from whichever goroutine reads the connection.

// DefaultMediaReceiver is the application every sender knows how to drive. Supporting it is what
// makes this device something you can cast a url to from anything.
const DefaultMediaReceiver = "CC1AD845"

// Playing is what the host has to do to actually make a sound. Nothing here touches audio; the
// receiver decides what should be happening and this is how it says so.
//
// Nil is fine and leaves a receiver that negotiates correctly and plays nothing, which is what it
// does until the speaker is wired in.
type Playing interface {
	// Load is a new thing to play, and whether to start it now.
	Load(m Media, at time.Duration, play bool) error

	// Play, Pause and Stop are the transport.
	Play() error
	Pause() error
	Stop() error

	// Seek moves within what is loaded.
	Seek(to time.Duration) error

	// Elapsed is how far in it is, which a status reports.
	Elapsed() time.Duration
}

// Receiver is this device as something to cast to.
type Receiver struct {
	// Name is what a sender shows. It goes in the mDNS record too, and the two have to agree or a
	// sender lists one name and connects to a device calling itself another.
	Name string

	// Player is what actually makes a sound. Nil negotiates and plays nothing.
	Player Playing

	// Volume is called when a sender moves the volume, as a fraction from nought to one.
	Volume func(level float64, muted bool)

	// Launched and Stopped bracket an application running.
	Launched func(app string)
	Stopped  func(app string)

	// Loaded is what a sender asked to play, before the player sees it.
	Loaded func(m Media)

	// Unspoken is a message on a namespace this does not speak, by its type; binary for protobuf.
	Unspoken func(namespace, kind string)

	// Credentials are the current ones, or nil while there are none.
	Credentials func() *Credentials

	// CRL is the device revocation list to answer a challenge with, or nil while there is none.
	CRL func() []byte

	// Challenged is a sender asking for device authentication, and whether it was answered.
	Challenged func(c Challenge, answered bool)

	// Available says whether an application can run here. Nil is every application.
	Available func(app string) bool

	// Identity is the device as advertised, which GET_DEVICE_INFO is answered from.
	Identity func() Device

	// Eureka is the device description a setup request is answered with.
	Eureka func() Eureka

	// Setup is every setup request, by type, with the data it carried.
	Setup func(kind string, data json.RawMessage)

	volume Volume

	// protocols are the registered application protocols, by name.
	protocols map[string]Protocol

	// app is what is running, or nil. One at a time: this device is a speaker, and two
	// applications wanting the same speaker is a fight rather than a feature.
	app *Application

	// media is what is loaded under it.
	media   *Media
	state   string
	idle    string
	session int

	// senders is who has opened a conversation, so a status can be sent to each rather than
	// broadcast blindly. The key is the sender's own endpoint id.
	senders map[string]bool

	// sessions counts up, so every launch and every load gets an id nothing else has used.
	sessions int
}

// NewReceiver is a receiver answering to a name, with nothing running.
func NewReceiver(name string) *Receiver {
	return &Receiver{
		Name:    name,
		state:   StateIdle,
		senders: map[string]bool{},
		volume: Volume{
			Level:        ptr(1.0),
			Muted:        ptr(false),
			StepInterval: 0.05,
			ControlType:  "attenuation",
		},
	}
}

func ptr[T any](v T) *T { return &v }

// Status is what the receiver would report now.
func (r *Receiver) Status() ReceiverStatus {
	s := ReceiverStatus{Volume: r.volume}
	if r.app != nil {
		s.Applications = []Application{*r.app}
	}
	return s
}

// Running is the application id, or empty.
func (r *Receiver) Running() string {
	if r.app == nil {
		return ""
	}
	return r.app.AppID
}

// State is what the player is doing, for a caller drawing it.
func (r *Receiver) State() string { return r.state }

// Media is what is loaded, or nil.
func (r *Receiver) Media() *Media { return r.media }

// Receive takes one message and returns what to send back.
//
// An error is a message that could not be understood, not a broken connection: the caller logs it
// and reads the next one. Dropping the connection over one bad payload turns a sender's bug into a
// device that cannot be cast to.
func (r *Receiver) Receive(m Message) ([]Message, error) {
	switch m.Namespace {
	case NSConnection:
		return r.connection(m)
	case NSHeartbeat:
		return r.heartbeat(m)
	case NSReceiver:
		return r.receiver(m)
	case NSMedia:
		return r.mediaNamespace(m)
	case NSDeviceAuth:
		return r.deviceAuth(m)
	case NSSetup:
		return r.setup(m)
	case NSDiscovery:
		return r.discovery(m)
	}

	if p := r.protocol(m.Namespace); p != nil {
		out, err := p.Receive(r.app, m)
		if !errors.Is(err, ErrUnspoken) {
			return out, err
		}
	}

	// A namespace this does not speak. Nothing to say: the receiver status already lists what the
	// running application answers, and a sender that ignored it is not owed a second answer.
	if r.Unspoken != nil {
		kind := "binary"
		if len(m.Binary) == 0 {
			if h, err := Kind(m.Payload); err == nil {
				kind = h.Type
			}
		}
		r.Unspoken(m.Namespace, kind)
	}
	return nil, nil
}

// deviceAuth answers a challenge from the current credentials, whichever made them.
func (r *Receiver) deviceAuth(m Message) ([]Message, error) {
	c, err := ParseChallenge(m.Binary)
	if err != nil {
		return nil, err
	}

	var creds *Credentials
	if r.Credentials != nil {
		creds = r.Credentials()
	}

	var body []byte
	switch {
	case creds == nil:
		body = Refuse(AuthInternal)
	case c.Algorithm != PKCS1v15:
		body = Refuse(AuthNoAlgorithm)
	default:
		var crl []byte
		if r.CRL != nil {
			crl = r.CRL()
		}
		body = Answer(creds, c.Hash, crl)
	}
	if r.Challenged != nil {
		r.Challenged(c, creds != nil && c.Algorithm == PKCS1v15)
	}

	return []Message{{
		Source:      m.Destination,
		Destination: m.Source,
		Namespace:   NSDeviceAuth,
		Binary:      body,
	}}, nil
}

// reply is a message back to whoever sent one.
func reply(to Message, namespace, body string) Message { return Reply(to, namespace, body) }

// Reply is a message back to whoever sent one, from whoever it was sent to.
func Reply(to Message, namespace, body string) Message {
	return Message{
		Source:      to.Destination,
		Destination: to.Source,
		Namespace:   namespace,
		Payload:     body,
	}
}

// connection opens and closes a sender's conversation.
//
// Neither is answered. CONNECT has no reply in the protocol, and a receiver that invents one is
// sending a sender something it has no case for.
func (r *Receiver) connection(m Message) ([]Message, error) {
	h, err := Kind(m.Payload)
	if err != nil {
		return nil, err
	}

	switch h.Type {
	case TypeConnect:
		r.senders[m.Source] = true
	case TypeClose:
		delete(r.senders, m.Source)
	}
	return nil, nil
}

// heartbeat answers a ping.
//
// The cheapest message here and the one that must never be missed: a sender that does not get a
// pong inside its own timeout drops the connection, and the music stops for a reason nothing logs.
func (r *Receiver) heartbeat(m Message) ([]Message, error) {
	h, err := Kind(m.Payload)
	if err != nil {
		return nil, err
	}
	if h.Type != TypePing {
		return nil, nil
	}
	return []Message{reply(m, NSHeartbeat, Pong())}, nil
}

// receiver answers the namespace about the device itself.
func (r *Receiver) receiver(m Message) ([]Message, error) {
	h, err := Kind(m.Payload)
	if err != nil {
		return nil, err
	}

	switch h.Type {
	case TypeGetStatus:
		return []Message{reply(m, NSReceiver, Status(h.RequestID, r.Status()))}, nil

	case TypeLaunch:
		return r.launch(m, h)

	case TypeStop:
		return r.stop(m, h)

	case TypeSetVolume:
		return r.setVolume(m, h)

	case TypeGetAppAvailability:
		return r.availability(m)
	}

	return []Message{reply(m, NSReceiver, Invalid(h.RequestID, ReasonInvalidCommand))}, nil
}

// launch starts an application.
//
// Answered with a receiver status rather than with a type of its own. That is what the protocol
// says and what senders wait for: one that answers "LAUNCHED" is a device that launches and then
// appears to hang.
func (r *Receiver) launch(m Message, h Header) ([]Message, error) {
	req, err := ParseLaunch(m.Payload)
	if err != nil {
		return []Message{reply(m, NSReceiver, Invalid(h.RequestID, ReasonInvalidParams))}, nil
	}

	// Any application id is taken, and answered with the one behaviour this device has.
	//
	// A real Chromecast runs a web application per id and refuses the ones it does not have. This
	// device has nothing to launch, so refusing would only be an imitation of that — and the
	// senders that matter do not read a refusal as a reason to try something else. Music
	// Assistant's cast provider launches its own id, C35B0678, and on a refusal raises rather than
	// falling back, telling the person to turn a setting off instead.
	//
	// Safe to accept because what follows does not depend on the id: after the launch it sends
	// LOAD on the standard media namespace, which is the one implemented here. The namespaces
	// reported back are still only the ones actually spoken, so a sender looking for an
	// application's own protocol can see it is not on offer.

	// Already running it. Not an error — a second sender joining the same application is ordinary
	// — so it gets the status of what is there rather than a new session.
	if r.app == nil || r.app.AppID != req.AppID {
		r.release(IdleInterrupted)
		r.start(req.AppID)
	}

	return r.announce(m, h.RequestID), nil
}

// start puts an application on.
// sessionID is a random UUID, which is what real receivers use for a session and its transport.
func sessionID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (r *Receiver) start(app string) {
	id := sessionID()
	known := Lookup(app)

	r.app = &Application{
		AppID:          app,
		UniversalAppID: app,
		AppType:        "WEB",
		IconURL:        known.Icon,
		SessionID:      id,

		// The same string as the session. A sender addresses the application by the transport id,
		// so the two being different is legal and being the same is what every real device does.
		TransportID: id,

		DisplayName: known.Name,
		StatusText:  known.Name,
		Namespaces:  r.namespaces(known),
	}
	r.bracket(known, true)

	if r.Launched != nil {
		r.Launched(app)
	}
}

// stop takes it away.
func (r *Receiver) stop(m Message, h Header) ([]Message, error) {
	req, err := ParseStop(m.Payload)
	if err != nil {
		return []Message{reply(m, NSReceiver, Invalid(h.RequestID, ReasonInvalidParams))}, nil
	}

	// A stop naming a session that is not the one running is a sender that has lost track, and
	// stopping anyway would take the music off for somebody else in the room.
	if req.SessionID != "" && (r.app == nil || r.app.SessionID != req.SessionID) {
		return []Message{reply(m, NSReceiver, Invalid(h.RequestID, ReasonInvalidParams))}, nil
	}

	r.release(IdleCancelled)
	return r.announce(m, h.RequestID), nil
}

// End stops the running application from this end, and is the status every sender is owed.
func (r *Receiver) End() []Message {
	if r.app == nil {
		return nil
	}
	r.release(IdleCancelled)
	out := make([]Message, 0, len(r.senders))
	for id := range r.senders {
		out = append(out, Message{Source: ReceiverID, Destination: id, Namespace: NSReceiver, Payload: Status(0, r.Status())})
	}
	return out
}

// release stops whatever is running and says why the player went idle.
func (r *Receiver) release(why string) {
	if r.app == nil {
		return
	}

	if r.Player != nil {
		r.Player.Stop()
	}

	app := r.app.AppID
	r.app = nil
	r.media = nil
	r.state = StateIdle
	r.idle = why
	r.session = 0
	r.bracket(Lookup(app), false)

	if r.Stopped != nil {
		r.Stopped(app)
	}
}

// setVolume moves the volume.
func (r *Receiver) setVolume(m Message, h Header) ([]Message, error) {
	req, err := ParseSetVolume(m.Payload)
	if err != nil {
		return []Message{reply(m, NSReceiver, Invalid(h.RequestID, ReasonInvalidParams))}, nil
	}

	// Only what was asked for. A request carrying a mute and no level must not set the level, and
	// the two arrive separately all the time.
	if req.Volume.Level != nil {
		r.volume.Level = ptr(*req.Volume.Level)
	}
	if req.Volume.Muted != nil {
		r.volume.Muted = ptr(*req.Volume.Muted)
	}

	if r.Volume != nil {
		r.Volume(*r.volume.Level, *r.volume.Muted)
	}

	return r.announce(m, h.RequestID), nil
}

// Report takes the volume the device is at, and returns the status every sender is owed when it is news.
func (r *Receiver) Report(level float64, muted bool) []Message {
	if *r.volume.Level == level && *r.volume.Muted == muted {
		return nil
	}
	r.volume.Level, r.volume.Muted = ptr(level), ptr(muted)

	var out []Message
	for id := range r.senders {
		out = append(out, Message{
			Source:      ReceiverID,
			Destination: id,
			Namespace:   NSReceiver,
			Payload:     Status(0, r.Status()),
		})
	}
	return out
}

// announce answers the sender that asked and tells everyone else.
//
// Both, because a change one sender made has to reach the others: two phones with the app open
// should not disagree about whether anything is playing.
func (r *Receiver) announce(m Message, request int) []Message {
	out := []Message{reply(m, NSReceiver, Status(request, r.Status()))}

	for id := range r.senders {
		if id == m.Source {
			continue
		}
		out = append(out, Message{
			Source:      ReceiverID,
			Destination: id,
			Namespace:   NSReceiver,
			Payload:     Status(0, r.Status()),
		})
	}
	return out
}

// mediaNamespace answers the namespace about what is playing.
func (r *Receiver) mediaNamespace(m Message) ([]Message, error) {
	h, err := Kind(m.Payload)
	if err != nil {
		return nil, err
	}

	// Nothing is running, so there is nothing to command. A sender that got here has a stale
	// transport id, and an empty status is what tells it so.
	if r.app == nil && h.Type != TypeGetStatus {
		return []Message{reply(m, NSMedia, Invalid(h.RequestID, ReasonInvalidCommand))}, nil
	}

	switch h.Type {
	case TypeLoad:
		return r.load(m, h)
	case TypeGetStatus:
		return []Message{reply(m, NSMedia, MediaStatusPayload(h.RequestID, r.playing()...))}, nil
	case TypePlay, TypePause, TypeStopMedia, TypeSeek:
		return r.transport(m, h)
	}

	return []Message{reply(m, NSMedia, Invalid(h.RequestID, ReasonInvalidCommand))}, nil
}

// load takes something new to play.
func (r *Receiver) load(m Message, h Header) ([]Message, error) {
	req, err := ParseLoad(m.Payload)
	if err != nil {
		return []Message{reply(m, NSMedia, LoadFailed(h.RequestID))}, nil
	}
	if r.Loaded != nil {
		r.Loaded(req.Media)
	}

	r.sessions++
	r.session = r.sessions
	r.media = &req.Media
	r.idle = ""

	at := seconds(req.CurrentTime)

	if r.Player != nil {
		if err := r.Player.Load(req.Media, at, req.Starts()); err != nil {
			r.media = nil
			r.session = 0
			r.state = StateIdle
			r.idle = IdleError
			return []Message{reply(m, NSMedia, LoadFailed(h.RequestID))},
				fmt.Errorf("cast: loading %q: %w", req.Media.ContentID, err)
		}
	}

	r.state = StatePlaying
	if !req.Starts() {
		r.state = StatePaused
	}

	return r.report(m, h.RequestID), nil
}

// transport is play, pause, stop or seek.
func (r *Receiver) transport(m Message, h Header) ([]Message, error) {
	req, err := ParseMediaRequest(m.Payload)
	if err != nil {
		return []Message{reply(m, NSMedia, Invalid(h.RequestID, ReasonInvalidParams))}, nil
	}

	// A command for a session that is not the one loaded is a sender that has lost track. Acting
	// on it would pause music somebody else started.
	if req.MediaSessionID != 0 && req.MediaSessionID != r.session {
		return []Message{reply(m, NSMedia, Invalid(h.RequestID, ReasonInvalidParams))}, nil
	}

	var act error
	switch h.Type {
	case TypePlay:
		r.state = StatePlaying
		if r.Player != nil {
			act = r.Player.Play()
		}

	case TypePause:
		r.state = StatePaused
		if r.Player != nil {
			act = r.Player.Pause()
		}

	case TypeStopMedia:
		// Stopping the media is not stopping the application: the sender can load something else
		// without launching again.
		if r.Player != nil {
			act = r.Player.Stop()
		}
		r.media = nil
		r.session = 0
		r.state = StateIdle
		r.idle = IdleCancelled

	case TypeSeek:
		if r.Player != nil {
			act = r.Player.Seek(req.Seek())
		}
		switch req.ResumeState {
		case ResumePlay:
			r.state = StatePlaying
		case ResumePause:
			r.state = StatePaused
		}
	}

	if act != nil {
		return []Message{reply(m, NSMedia, Invalid(h.RequestID, ReasonInvalidCommand))},
			fmt.Errorf("cast: %s: %w", h.Type, act)
	}
	return r.report(m, h.RequestID), nil
}

// playing is the media status, or nothing at all when there is none.
func (r *Receiver) playing() []MediaStatus {
	if r.session == 0 {
		return nil
	}

	s := MediaStatus{
		MediaSessionID:         r.session,
		PlaybackRate:           1,
		PlayerState:            r.state,
		SupportedMediaCommands: Speaker,
		Volume:                 r.volume,
		IdleReason:             r.idle,
	}

	if r.Player != nil {
		s.CurrentTime = asSeconds(r.Player.Elapsed())
	}
	return []MediaStatus{s}
}

// report answers with the media status and tells the other senders.
//
// The media itself goes only to the answer. It is a few kilobytes with artwork urls and it has not
// changed for anybody who was already watching.
func (r *Receiver) report(m Message, request int) []Message {
	full := r.playing()
	if len(full) > 0 {
		full[0].Media = r.media
	}

	out := []Message{reply(m, NSMedia, MediaStatusPayload(request, full...))}

	brief := r.playing()
	for id := range r.senders {
		if id == m.Source {
			continue
		}
		out = append(out, Message{
			Source:      m.Destination,
			Destination: id,
			Namespace:   NSMedia,
			Payload:     MediaStatusPayload(0, brief...),
		})
	}
	return out
}

// Finished is the player having reached the end on its own, which nothing asked for and every
// sender wants to hear about.
func (r *Receiver) Finished() []Message {
	if r.session == 0 {
		return nil
	}

	r.state = StateIdle
	r.idle = IdleFinished
	r.media = nil

	status := r.playing()
	r.session = 0

	var out []Message
	for id := range r.senders {
		out = append(out, Message{
			Source:      ReceiverID,
			Destination: id,
			Namespace:   NSMedia,
			Payload:     MediaStatusPayload(0, status...),
		})
	}
	return out
}

// Forget drops everything, for a connection that has gone away.
func (r *Receiver) Forget() {
	r.release(IdleCancelled)
	clear(r.senders)
	r.idle = ""
}
