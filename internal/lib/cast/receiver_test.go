package cast

import (
	"encoding/json"
	"testing"
	"time"
)

// A sender, scripted. Everything goes through Receive the way it would off a real connection.

// from builds a message as a sender would send it.
func from(sender, namespace, body string) Message {
	return Message{
		Source:      sender,
		Destination: ReceiverID,
		Namespace:   namespace,
		Payload:     body,
	}
}

// toApp is a message addressed to a running application rather than to the receiver.
func toApp(sender, transport, namespace, body string) Message {
	return Message{
		Source:      sender,
		Destination: transport,
		Namespace:   namespace,
		Payload:     body,
	}
}

// answered is the one message back, and fails if there was not exactly one.
func answered(t *testing.T, r *Receiver, m Message) Message {
	t.Helper()

	out, err := r.Receive(m)
	if err != nil {
		t.Fatalf("%s: %v", m.Namespace, err)
	}
	if len(out) != 1 {
		t.Fatalf("%s gave %d messages, want one", m.Namespace, len(out))
	}
	return out[0]
}

// replyTo is the message back to whoever sent one, ignoring any broadcast to the others.
func replyTo(t *testing.T, r *Receiver, m Message) Message {
	t.Helper()

	out, err := r.Receive(m)
	if err != nil {
		t.Fatalf("%s: %v", m.Namespace, err)
	}
	for _, a := range out {
		if a.Destination == m.Source {
			return a
		}
	}

	t.Fatalf("%s: nothing came back to %s", m.Namespace, m.Source)
	return Message{}
}

// status reads a receiver status out of a payload.
func status(t *testing.T, payload string) ReceiverStatus {
	t.Helper()

	var got struct {
		Status ReceiverStatus `json:"status"`
	}
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("the status did not parse: %v", err)
	}
	return got.Status
}

// mediaStatus reads a media status list out of a payload.
func mediaStatus(t *testing.T, payload string) []MediaStatus {
	t.Helper()

	var got struct {
		Status []MediaStatus `json:"status"`
	}
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("the media status did not parse: %v", err)
	}
	return got.Status
}

// player records what the receiver asked it to do.
type player struct {
	loaded  *Media
	at      time.Duration
	started bool
	playing bool
	stopped int
	sought  time.Duration
	fail    error
}

func (p *player) Load(m Media, at time.Duration, play bool) error {
	if p.fail != nil {
		return p.fail
	}
	p.loaded, p.at, p.started, p.playing = &m, at, play, play
	return nil
}

func (p *player) Play() error  { p.playing = true; return nil }
func (p *player) Pause() error { p.playing = false; return nil }

func (p *player) Stop() error {
	p.playing = false
	p.stopped++
	return nil
}

func (p *player) Seek(to time.Duration) error { p.sought = to; return nil }
func (p *player) Elapsed() time.Duration      { return p.sought }

// The whole thing, in the order a sender does it: connect, look, launch, load, play, stop.
func TestASenderCastsSomethingAndControlsIt(t *testing.T) {
	p := &player{}
	r := NewReceiver("Kitchen")
	r.Player = p

	var launched, stopped string
	r.Launched = func(app string) { launched = app }
	r.Stopped = func(app string) { stopped = app }

	// Connect, which is not answered.
	if out, err := r.Receive(from(SenderID, NSConnection, Connect())); err != nil || len(out) != 0 {
		t.Fatalf("connect gave %d messages, %v", len(out), err)
	}

	// The heartbeat, which must never be missed.
	pong := answered(t, r, from(SenderID, NSHeartbeat, Ping()))
	if h, _ := Kind(pong.Payload); h.Type != TypePong {
		t.Fatalf("a ping was answered with %q", h.Type)
	}

	// What are you doing? Nothing.
	first := answered(t, r, from(SenderID, NSReceiver, `{"type":"GET_STATUS","requestId":1}`))
	if got := status(t, first.Payload); len(got.Applications) != 0 {
		t.Errorf("a fresh receiver says %d applications are running", len(got.Applications))
	}

	// Launch, which is answered with a receiver status and not a type of its own.
	out := answered(t, r, from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":2,"appId":"`+DefaultMediaReceiver+`"}`))

	h, err := Kind(out.Payload)
	if err != nil {
		t.Fatalf("Kind: %v", err)
	}
	if h.Type != TypeStatus {
		t.Fatalf("a launch was answered with %q, want a receiver status", h.Type)
	}
	if h.RequestID != 2 {
		t.Errorf("answered request %d, want 2", h.RequestID)
	}
	if launched != DefaultMediaReceiver {
		t.Errorf("launched %q", launched)
	}

	app := status(t, out.Payload).Applications
	if len(app) != 1 {
		t.Fatalf("%d applications after a launch", len(app))
	}
	transport := app[0].TransportID
	if transport == "" {
		t.Fatal("the application has no transport id, so nothing can be addressed to it")
	}

	// It has to say it speaks the media namespace, or a sender never tries.
	var speaks bool
	for _, n := range app[0].Namespaces {
		if n.Name == NSMedia {
			speaks = true
		}
	}
	if !speaks {
		t.Error("the application does not claim the media namespace")
	}

	// Load something.
	loaded := answered(t, r, toApp(SenderID, transport, NSMedia, `{
		"type":"LOAD","requestId":3,
		"media":{"contentId":"http://example/track.mp3","streamType":"BUFFERED","contentType":"audio/mpeg",
		         "metadata":{"metadataType":3,"title":"Sun Ra","artist":"Arkestra"},"duration":215}
	}`))

	if h, _ := Kind(loaded.Payload); h.Type != TypeMediaStatus {
		t.Fatalf("a load was answered with %q", h.Type)
	}
	if p.loaded == nil || p.loaded.ContentID != "http://example/track.mp3" {
		t.Fatalf("the player was given %+v", p.loaded)
	}
	if !p.started {
		t.Error("the load did not start playing, although autoplay was not set to false")
	}

	list := mediaStatus(t, loaded.Payload)
	if len(list) != 1 {
		t.Fatalf("%d statuses", len(list))
	}
	if list[0].PlayerState != StatePlaying {
		t.Errorf("state %q", list[0].PlayerState)
	}
	if list[0].Media == nil || list[0].Media.Metadata.Title != "Sun Ra" {
		t.Error("the status after a load does not carry the media")
	}
	session := list[0].MediaSessionID
	if session == 0 {
		t.Fatal("the load produced no session id")
	}

	// Pause, addressed to that session.
	paused := answered(t, r, toApp(SenderID, transport, NSMedia,
		`{"type":"PAUSE","requestId":4,"mediaSessionId":`+itoa(session)+`}`))

	if p.playing {
		t.Error("the player is still playing after a pause")
	}
	if got := mediaStatus(t, paused.Payload); got[0].PlayerState != StatePaused {
		t.Errorf("state %q after a pause", got[0].PlayerState)
	}

	// And stop the application.
	answered(t, r, from(SenderID, NSReceiver, `{"type":"STOP","requestId":5,"sessionId":"`+app[0].SessionID+`"}`))
	if stopped != DefaultMediaReceiver {
		t.Errorf("stopped %q", stopped)
	}
	if r.Running() != "" {
		t.Errorf("%q is still running", r.Running())
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}

	var out []byte
	for v > 0 {
		out = append([]byte{byte('0' + v%10)}, out...)
		v /= 10
	}
	return string(out)
}

// A sender that asks for its own application gets it, and gets told that is what is running.
//
// This device has one behaviour and nothing to launch, so an id it does not recognise is not a
// reason to refuse: senders do not read a refusal as an invitation to try the default. Music
// Assistant asks for C35B0678 and, refused, raises rather than falling back — it tells the person
// to turn a setting off. What it does next does not depend on the id either, since it sends LOAD
// on the standard media namespace.
func TestAnyApplicationIsAccepted(t *testing.T) {
	for _, app := range []string{DefaultMediaReceiver, "C35B0678", "DD107DDB", "DEADBEEF"} {
		r := NewReceiver("Kitchen")

		out := answered(t, r, from(SenderID, NSReceiver,
			`{"type":"LAUNCH","requestId":1,"appId":"`+app+`"}`))

		h, _ := Kind(out.Payload)
		if h.Type != TypeStatus {
			t.Errorf("%s came back %q, want a receiver status", app, h.Type)
			continue
		}
		if got := r.Running(); got != app {
			t.Errorf("%s is running %q, want the id that was asked for", app, got)
		}

		// Reported as running, because a sender checks that before it sends anything.
		apps := status(t, out.Payload).Applications
		if len(apps) != 1 || apps[0].AppID != app {
			t.Errorf("%s: the status does not name it as running: %+v", app, apps)
		}
	}
}

// What is offered stays honest: only the namespaces this device actually speaks are reported, so
// a sender looking for an application's own protocol can see it is not there.
func TestAcceptingAnAppDoesNotPromiseItsNamespaces(t *testing.T) {
	r := NewReceiver("Kitchen")

	out := answered(t, r, from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"C35B0678"}`))

	apps := status(t, out.Payload).Applications
	if len(apps) != 1 {
		t.Fatalf("want one application, got %+v", apps)
	}

	for _, n := range apps[0].Namespaces {
		switch n.Name {
		case NSConnection, NSHeartbeat, NSMedia:
		default:
			t.Errorf("offers %q, which this device does not speak", n.Name)
		}
	}
}

// A second sender joining the application already running is ordinary, and must not restart it —
// that would take the music off for whoever started it.
func TestASecondSenderJoinsRatherThanRestarts(t *testing.T) {
	r := NewReceiver("Kitchen")

	first := answered(t, r, from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	was := status(t, first.Payload).Applications[0].SessionID

	r.Receive(from("sender-1", NSConnection, Connect()))

	out, err := r.Receive(from("sender-1", NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	if err != nil {
		t.Fatalf("the second launch: %v", err)
	}

	now := status(t, out[0].Payload).Applications[0].SessionID
	if now != was {
		t.Errorf("the application restarted: session %q became %q", was, now)
	}
}

// A change one sender made has to reach the others, or two phones with the app open disagree about
// whether anything is playing.
func TestTheOtherSendersAreToldAboutAChange(t *testing.T) {
	r := NewReceiver("Kitchen")

	r.Receive(from(SenderID, NSConnection, Connect()))
	r.Receive(from("sender-1", NSConnection, Connect()))

	out, err := r.Receive(from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("%d messages, want the answer and one broadcast", len(out))
	}

	var told bool
	for _, m := range out {
		if m.Destination != "sender-1" {
			continue
		}
		told = true

		h, _ := Kind(m.Payload)
		if h.RequestID != 0 {
			t.Errorf("the broadcast carries request %d, want none", h.RequestID)
		}
	}
	if !told {
		t.Error("the other sender was not told")
	}
}

// Level and mute arrive separately all the time, and setting the other to zero is the difference
// between muting a speaker and turning it all the way down.
func TestAMuteDoesNotMoveTheLevel(t *testing.T) {
	r := NewReceiver("Kitchen")

	var gotLevel float64
	var gotMuted bool
	r.Volume = func(level float64, muted bool) { gotLevel, gotMuted = level, muted }

	out := answered(t, r, from(SenderID, NSReceiver,
		`{"type":"SET_VOLUME","requestId":1,"volume":{"level":0.4}}`))
	if v := status(t, out.Payload).Volume; v.Level == nil || *v.Level != 0.4 {
		t.Fatalf("the level came back %v", v.Level)
	}

	out = answered(t, r, from(SenderID, NSReceiver,
		`{"type":"SET_VOLUME","requestId":2,"volume":{"muted":true}}`))

	v := status(t, out.Payload).Volume
	if v.Level == nil || *v.Level != 0.4 {
		t.Errorf("muting moved the level to %v", v.Level)
	}
	if v.Muted == nil || !*v.Muted {
		t.Error("the mute did not take")
	}
	if gotLevel != 0.4 || !gotMuted {
		t.Errorf("the host was told level %v muted %v", gotLevel, gotMuted)
	}
}

// A sender that has lost track and names a session that is not the one loaded must not be allowed
// to pause music somebody else started.
func TestACommandForTheWrongSessionIsRefused(t *testing.T) {
	r := NewReceiver("Kitchen")
	p := &player{}
	r.Player = p

	out := answered(t, r, from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	transport := status(t, out.Payload).Applications[0].TransportID

	answered(t, r, toApp(SenderID, transport, NSMedia,
		`{"type":"LOAD","requestId":2,"media":{"contentId":"http://example/a.mp3"}}`))

	refused := answered(t, r, toApp(SenderID, transport, NSMedia,
		`{"type":"PAUSE","requestId":3,"mediaSessionId":999}`))

	if h, _ := Kind(refused.Payload); h.Type != TypeInvalid {
		t.Errorf("came back %q, want a refusal", h.Type)
	}
	if !p.playing {
		t.Error("the wrong session's pause stopped the music")
	}
}

// Autoplay false is a sender asking for it loaded and paused, which is not the same as not sending
// the field at all.
func TestAutoplayFalseLoadsWithoutStarting(t *testing.T) {
	r := NewReceiver("Kitchen")
	p := &player{}
	r.Player = p

	out := answered(t, r, from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	transport := status(t, out.Payload).Applications[0].TransportID

	loaded := answered(t, r, toApp(SenderID, transport, NSMedia,
		`{"type":"LOAD","requestId":2,"autoplay":false,"media":{"contentId":"http://example/a.mp3"}}`))

	if p.started {
		t.Error("autoplay false started playing")
	}
	if got := mediaStatus(t, loaded.Payload); got[0].PlayerState != StatePaused {
		t.Errorf("state %q, want paused", got[0].PlayerState)
	}
}

// A load the player refuses is a LOAD_FAILED, and leaves nothing half loaded behind.
func TestALoadThePlayerRefuses(t *testing.T) {
	r := NewReceiver("Kitchen")
	p := &player{fail: errBroken}
	r.Player = p

	out := answered(t, r, from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	transport := status(t, out.Payload).Applications[0].TransportID

	answers, err := r.Receive(toApp(SenderID, transport, NSMedia,
		`{"type":"LOAD","requestId":2,"media":{"contentId":"http://example/a.mp3"}}`))
	if err == nil {
		t.Error("a load that failed reported success")
	}
	if len(answers) != 1 {
		t.Fatalf("%d answers", len(answers))
	}
	if h, _ := Kind(answers[0].Payload); h.Type != TypeLoadFailed {
		t.Errorf("came back %q, want a load failure", h.Type)
	}
	if r.Media() != nil {
		t.Error("something was left loaded")
	}
}

var errBroken = &castError{"nothing to play it with"}

type castError struct{ s string }

func (e *castError) Error() string { return e.s }

// Stopping the media is not stopping the application: a sender can load something else without
// launching again.
func TestStoppingTheMediaLeavesTheApplicationUp(t *testing.T) {
	r := NewReceiver("Kitchen")
	r.Player = &player{}

	out := answered(t, r, from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	transport := status(t, out.Payload).Applications[0].TransportID

	answered(t, r, toApp(SenderID, transport, NSMedia,
		`{"type":"LOAD","requestId":2,"media":{"contentId":"http://example/a.mp3"}}`))

	answered(t, r, toApp(SenderID, transport, NSMedia, `{"type":"STOP","requestId":3}`))

	if r.Running() == "" {
		t.Error("stopping the media stopped the application")
	}
	if r.Media() != nil {
		t.Error("the media is still loaded")
	}
}

// The end of a track is something nothing asked for and every sender wants to hear about.
func TestFinishingTellsEverySender(t *testing.T) {
	r := NewReceiver("Kitchen")
	r.Player = &player{}

	r.Receive(from(SenderID, NSConnection, Connect()))
	r.Receive(from("sender-1", NSConnection, Connect()))

	out := replyTo(t, r, from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	transport := status(t, out.Payload).Applications[0].TransportID

	r.Receive(toApp(SenderID, transport, NSMedia,
		`{"type":"LOAD","requestId":2,"media":{"contentId":"http://example/a.mp3"}}`))

	told := r.Finished()
	if len(told) != 2 {
		t.Fatalf("%d senders told, want both", len(told))
	}

	for _, m := range told {
		got := mediaStatus(t, m.Payload)
		if len(got) != 1 {
			t.Fatalf("%d statuses", len(got))
		}
		if got[0].PlayerState != StateIdle {
			t.Errorf("state %q, want idle", got[0].PlayerState)
		}
		if got[0].IdleReason != IdleFinished {
			t.Errorf("idle reason %q, want finished", got[0].IdleReason)
		}
	}
}

// Nothing running means nothing to command, and a sender with a stale transport id has to be told
// rather than left waiting.
func TestACommandWithNothingRunning(t *testing.T) {
	r := NewReceiver("Kitchen")

	out := answered(t, r, toApp(SenderID, "1", NSMedia, `{"type":"PAUSE","requestId":1}`))
	if h, _ := Kind(out.Payload); h.Type != TypeInvalid {
		t.Errorf("came back %q, want a refusal", h.Type)
	}
}

// Asking what is playing when nothing is is a fair question with an empty answer, not a refusal.
func TestAskingWhatIsPlayingWhenNothingIs(t *testing.T) {
	r := NewReceiver("Kitchen")

	out := answered(t, r, toApp(SenderID, "1", NSMedia, `{"type":"GET_STATUS","requestId":1}`))

	h, _ := Kind(out.Payload)
	if h.Type != TypeMediaStatus {
		t.Fatalf("came back %q", h.Type)
	}
	if got := mediaStatus(t, out.Payload); len(got) != 0 {
		t.Errorf("%d statuses, want none", len(got))
	}
}

// A payload that is not JSON is a sender's bug, and dropping the connection over it turns that into
// a device that cannot be cast to.
func TestRubbishIsAnErrorAndNotAnAnswer(t *testing.T) {
	r := NewReceiver("Kitchen")

	out, err := r.Receive(from(SenderID, NSReceiver, `not json at all`))
	if err == nil {
		t.Error("rubbish was accepted")
	}
	if len(out) != 0 {
		t.Errorf("%d messages came back", len(out))
	}
}

// A namespace this does not speak is not answered: the status already listed what is on offer.
func TestANamespaceThisDoesNotSpeak(t *testing.T) {
	r := NewReceiver("Kitchen")

	out, err := r.Receive(from(SenderID, "urn:x-cast:com.example.whatever", `{"type":"HELLO"}`))
	if err != nil {
		t.Errorf("gave %v", err)
	}
	if len(out) != 0 {
		t.Errorf("%d messages came back", len(out))
	}
}

// A sender that has gone away stops being told about changes.
func TestAClosedSenderIsNotToldAnyMore(t *testing.T) {
	r := NewReceiver("Kitchen")

	r.Receive(from(SenderID, NSConnection, Connect()))
	r.Receive(from("sender-1", NSConnection, Connect()))
	r.Receive(from("sender-1", NSConnection, Close()))

	out, err := r.Receive(from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("%d messages, want only the answer", len(out))
	}
}

// A seek moves the player and may say what to do about playing.
func TestSeeking(t *testing.T) {
	r := NewReceiver("Kitchen")
	p := &player{}
	r.Player = p

	out := answered(t, r, from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	transport := status(t, out.Payload).Applications[0].TransportID

	answered(t, r, toApp(SenderID, transport, NSMedia,
		`{"type":"LOAD","requestId":2,"media":{"contentId":"http://example/a.mp3"}}`))

	sought := answered(t, r, toApp(SenderID, transport, NSMedia,
		`{"type":"SEEK","requestId":3,"currentTime":42.5,"resumeState":"PLAYBACK_PAUSE"}`))

	if p.sought != 42500*time.Millisecond {
		t.Errorf("sought to %v, want 42.5s", p.sought)
	}
	if got := mediaStatus(t, sought.Payload); got[0].PlayerState != StatePaused {
		t.Errorf("state %q, want paused", got[0].PlayerState)
	}
}

// A sender greys out what the status does not claim, so this is the difference between a pause
// button that works and one that is not drawn.
func TestTheStatusSaysWhatCanBeDoneToIt(t *testing.T) {
	r := NewReceiver("Kitchen")
	r.Player = &player{}

	out := answered(t, r, from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	transport := status(t, out.Payload).Applications[0].TransportID

	loaded := answered(t, r, toApp(SenderID, transport, NSMedia,
		`{"type":"LOAD","requestId":2,"media":{"contentId":"http://example/a.mp3"}}`))

	got := mediaStatus(t, loaded.Payload)[0]
	for _, want := range []struct {
		name string
		bit  int
	}{
		{"pause", CommandPause},
		{"seek", CommandSeek},
		{"volume", CommandStreamVolume},
		{"mute", CommandStreamMute},
	} {
		if got.SupportedMediaCommands&want.bit == 0 {
			t.Errorf("the status does not offer %s", want.name)
		}
	}

	// And does not claim what is not implemented.
	if got.SupportedMediaCommands&CommandQueueNext != 0 {
		t.Error("the status claims a queue, which would get a next button that does nothing")
	}
}

// The media goes on the answer and not on the broadcast: it is a few kilobytes with artwork urls
// and it has not changed for anybody who was already watching.
func TestTheBroadcastLeavesTheMediaOut(t *testing.T) {
	r := NewReceiver("Kitchen")
	r.Player = &player{}

	r.Receive(from(SenderID, NSConnection, Connect()))
	r.Receive(from("sender-1", NSConnection, Connect()))

	out := replyTo(t, r, from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	transport := status(t, out.Payload).Applications[0].TransportID

	answers, err := r.Receive(toApp(SenderID, transport, NSMedia,
		`{"type":"LOAD","requestId":2,"media":{"contentId":"http://example/a.mp3","metadata":{"title":"x"}}}`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(answers) != 2 {
		t.Fatalf("%d messages, want the answer and one broadcast", len(answers))
	}

	for _, m := range answers {
		got := mediaStatus(t, m.Payload)[0]
		if m.Destination == SenderID && got.Media == nil {
			t.Error("the answer does not carry the media")
		}
		if m.Destination == "sender-1" && got.Media != nil {
			t.Error("the broadcast carries the media")
		}
	}
}

// Forgetting a connection leaves nothing behind for the next one.
func TestForgettingAConnection(t *testing.T) {
	r := NewReceiver("Kitchen")
	p := &player{}
	r.Player = p

	r.Receive(from(SenderID, NSConnection, Connect()))
	out := answered(t, r, from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	transport := status(t, out.Payload).Applications[0].TransportID

	answered(t, r, toApp(SenderID, transport, NSMedia,
		`{"type":"LOAD","requestId":2,"media":{"contentId":"http://example/a.mp3"}}`))

	r.Forget()

	if r.Running() != "" || r.Media() != nil {
		t.Errorf("state survived: running %q media %v", r.Running(), r.Media())
	}
	if p.stopped == 0 {
		t.Error("the player was not stopped")
	}

	// And the next launch is answered to that sender alone, since the old ones are gone.
	again, err := r.Receive(from("sender-2", NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	if err != nil {
		t.Fatalf("relaunch: %v", err)
	}
	if len(again) != 1 {
		t.Errorf("%d messages, want only the answer", len(again))
	}
}

func TestEndingFromThisEndTellsEverySender(t *testing.T) {
	r := NewReceiver("Kitchen")
	r.Receive(from(SenderID, NSConnection, Connect()))
	r.Receive(from("sender-1", NSConnection, Connect()))
	if _, err := r.Receive(from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`)); err != nil {
		t.Fatal(err)
	}

	out := r.End()
	if r.Running() != "" {
		t.Fatalf("still running %q after End", r.Running())
	}
	if len(out) != 2 {
		t.Fatalf("%d messages, want one for each of the two senders", len(out))
	}
	for _, m := range out {
		if apps := status(t, m.Payload).Applications; len(apps) != 0 {
			t.Errorf("%s was told %d applications are running", m.Destination, len(apps))
		}
	}
	if again := r.End(); again != nil {
		t.Errorf("ending with nothing running said %d things", len(again))
	}
}
