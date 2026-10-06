package call

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/message"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/lib/rtc"
	"github.com/ygelfand/LANovo/internal/lib/safe"
	"github.com/ygelfand/LANovo/internal/lib/say"
)

func init() {
	component.Register(component.Network, Get, component.Order(62))
	web.Handle("POST /call/offer", http.HandlerFunc(Get().offered))
	web.Handle("POST /call/answer", http.HandlerFunc(Get().answered))
	web.Handle("POST /call/end", http.HandlerFunc(Get().ended))
}

const (
	ringFor    = 45 * time.Second
	setupWait  = 10 * time.Second
	noticeFor  = 4 * time.Second
	clockEvery = time.Second
)

type State int

const (
	Idle State = iota
	Calling
	Ringing
	Talking
)

func (s State) String() string {
	return [...]string{"idle", "calling", "ringing", "talking"}[s]
}

type Reason string

const (
	ReasonHangup      Reason = "hangup"
	ReasonDeclined    Reason = "declined"
	ReasonCancelled   Reason = "cancelled"
	ReasonUnanswered  Reason = "unanswered"
	ReasonBusy        Reason = "busy"
	ReasonUnavailable Reason = "unavailable"
	ReasonDropped     Reason = "dropped"
	ReasonFailed      Reason = "failed"
)

var (
	ErrBusy    = errors.New("already in a call")
	ErrNoRoute = errors.New("peer has no IPv4 address")
)

type Call struct {
	ID              string
	Peer            discovery.Peer
	State           State
	Since           time.Time
	Muted           bool
	Video           bool
	CameraOff       bool
	Covered         bool
	RemoteMuted     bool
	RemoteCameraOff bool
}

type session struct {
	Call
	addr   string
	offer  string
	link   *rtc.Link
	remote livecam.Size
	own    livecam.Size
	pics   *pictures
	ctx    context.Context
	stop   context.CancelFunc
	quiet  context.CancelFunc
	hold   *shell.Hold
	unhook func()
}

type Calls struct {
	mu   sync.Mutex
	now  *session
	view View
	ha   entities
}

type View struct{}

func (*View) Covers() bool { return true }

func (*View) Wakes() bool { return true }

type Profile struct{ ID string }

func (*Profile) Covers() bool { return true }

var (
	once   sync.Once
	shared *Calls
)

func Get() *Calls {
	once.Do(func() {
		shared = &Calls{}
		shared.build()
	})
	return shared
}

func (c *Calls) Name() string { return "call" }

func (c *Calls) Screen() *View { return &c.view }

func (c *Calls) Now() (Call, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.now == nil {
		return Call{}, false
	}
	return c.now.Call, true
}

type Stats struct {
	Link   rtc.Stats
	Remote LayerStats
	Self   LayerStats
}

func (c *Calls) Stats() (Stats, bool) {
	c.mu.Lock()
	s := c.now
	var link *rtc.Link
	var pics *pictures
	if s != nil {
		link, pics = s.link, s.pics
	}
	c.mu.Unlock()
	if link == nil {
		return Stats{}, false
	}
	st := Stats{Link: link.Stats()}
	if pics != nil {
		st.Remote, st.Self = pics.remote.stats(), pics.self.stats()
	}
	return st, true
}

func (c *Calls) Talking() bool {
	call, ok := c.Now()
	return ok && call.State == Talking
}

func (c *Calls) PausesWake() bool { return c.Talking() && config.Get().Call.PauseWake }

func (c *Calls) Open(p discovery.Peer) { shell.Get().Push(&Profile{ID: p.ID}) }

func (c *Calls) Start(p discovery.Peer, video bool) error {
	addr, err := route(p)
	if err != nil {
		return err
	}
	c.mu.Lock()
	if c.now != nil {
		c.mu.Unlock()
		return ErrBusy
	}
	s := c.open(Call{ID: newID(), Peer: p, State: Calling, Since: time.Now(), Video: video}, addr)
	c.mu.Unlock()
	slog.Info("calling", "to", p.Name, "call", s.ID, "video", video)
	c.changed(s)
	safe.Go("call dial", func() { c.dial(s) })
	return nil
}

func (c *Calls) dial(s *session) {
	m, own := c.media(s)
	link, err := rtc.New(m, c.lost(s))
	if err != nil {
		c.fail(s, "link", err)
		return
	}
	c.mu.Lock()
	s.link = link
	c.mu.Unlock()
	c.apply(s)
	ctx, cancel := context.WithTimeout(s.ctx, setupWait)
	sdp, err := link.Offer(ctx)
	cancel()
	if err != nil {
		c.fail(s, "offer", err)
		return
	}
	me := discovery.Self()
	status, err := post(s.ctx, s.addr, "offer", envelope{Call: s.ID, From: me.ID, Name: me.Name, Model: me.Model, Board: me.Board, Caps: me.Caps, SDP: sdp, Video: s.Video, Width: own.Width, Height: own.Height})
	switch {
	case err != nil:
		c.fail(s, "offer", err)
		return
	case status == http.StatusConflict:
		c.close(s, ReasonBusy)
		return
	case status == http.StatusForbidden:
		c.close(s, ReasonUnavailable)
		return
	case status != http.StatusAccepted:
		c.fail(s, "offer", fmt.Errorf("status %d", status))
		return
	}
	ringing, quiet := context.WithCancel(s.ctx)
	c.mu.Lock()
	s.quiet = quiet
	c.mu.Unlock()
	safe.Go("call ringback", func() { ringback(ringing) })
	select {
	case <-s.ctx.Done():
	case <-time.After(ringFor):
		if c.state(s) == Calling {
			c.end(s, ReasonUnanswered)
		}
	}
}

func (c *Calls) Answer() {
	c.mu.Lock()
	s := c.now
	if s == nil || s.State != Ringing {
		c.mu.Unlock()
		return
	}
	s.quiet()
	c.mu.Unlock()
	safe.Go("call answer", func() { c.answer(s) })
}

func (c *Calls) answer(s *session) {
	m, own := c.media(s)
	link, err := rtc.New(m, c.lost(s))
	if err != nil {
		c.fail(s, "link", err)
		return
	}
	c.mu.Lock()
	s.link = link
	c.mu.Unlock()
	c.apply(s)
	ctx, cancel := context.WithTimeout(s.ctx, setupWait)
	sdp, err := link.Accept(ctx, s.offer)
	cancel()
	if err != nil {
		c.fail(s, "answer", err)
		return
	}
	status, err := post(s.ctx, s.addr, "answer", envelope{Call: s.ID, SDP: sdp, Width: own.Width, Height: own.Height})
	if err != nil || status != http.StatusOK {
		c.fail(s, "answer", fmt.Errorf("status %d: %v", status, err))
		return
	}
	c.talking(s)
}

func (c *Calls) AnswerAudio() {
	c.mu.Lock()
	if s := c.now; s != nil && s.State == Ringing {
		s.Video = false
	}
	c.mu.Unlock()
	c.Answer()
}

func (c *Calls) Hangup() {
	c.mu.Lock()
	s := c.now
	c.mu.Unlock()
	if s == nil {
		return
	}
	reason := ReasonHangup
	switch c.state(s) {
	case Ringing:
		reason = ReasonDeclined
	case Calling:
		reason = ReasonCancelled
	}
	c.end(s, reason)
}

func (c *Calls) Mute(on bool) {
	c.mu.Lock()
	s := c.now
	if s != nil {
		s.Muted = on
	}
	c.mu.Unlock()
	if s == nil {
		return
	}
	c.apply(s)
	c.changed(s)
}

func (c *Calls) Camera(on bool) {
	c.mu.Lock()
	s := c.now
	if s != nil {
		s.CameraOff = !on
	}
	c.mu.Unlock()
	if s == nil {
		return
	}
	c.apply(s)
	c.changed(s)
}

func (c *Calls) apply(s *session) {
	covered := privacy.Get().CameraCovered()
	silenced := privacy.Get().MicMuted()
	c.mu.Lock()
	s.Covered = covered
	link, pics := s.link, s.pics
	muted, blind := s.Muted || silenced, s.CameraOff || covered
	c.mu.Unlock()
	if link != nil {
		link.Mute(muted)
		link.CameraOff(blind)
		c.tell(s)
	}
	pics.showSelf(!blind)
}

func (c *Calls) media(s *session) (rtc.Media, livecam.Size) {
	m := rtc.Media{Audio: device{}, Opened: func() { c.tell(s); c.tellSize(s) }, Received: c.received(s)}
	c.mu.Lock()
	video, remote, off := s.Video, s.remote, s.CameraOff
	c.mu.Unlock()
	if !video {
		return m, livecam.Size{}
	}
	pics := c.newPictures(remote)
	m.Screen = pics.remote
	at, own, ok := cameraStream()
	c.mu.Lock()
	s.pics, s.own = pics, own
	c.mu.Unlock()
	if ok {
		m.Camera = camera{at: at}
		pics.start(s.ctx, c, s, at, own, !off && !privacy.Get().CameraCovered())
	}
	return m, own
}

func (c *Calls) open(call Call, addr string) *session {
	ctx, stop := context.WithCancel(context.Background())
	s := &session{Call: call, addr: addr, ctx: ctx, stop: stop, quiet: func() {}}
	s.Covered = privacy.Get().CameraCovered()
	s.unhook = privacy.Get().Changed.Listen(func(privacy.Marks) {
		c.apply(s)
		c.changed(s)
	})
	c.now = s
	return s
}

func (c *Calls) talking(s *session) {
	c.mu.Lock()
	if c.now != s {
		c.mu.Unlock()
		return
	}
	s.State, s.Since = Talking, time.Now()
	s.quiet()
	c.mu.Unlock()
	slog.Info("call connected", "peer", s.Peer.Name, "video", s.Video)
	media.Get().Pause()
	volume.Get().Sounding(config.StreamVoice)
	speaker.Sound().Claim("call", func(ctx context.Context, _ *speaker.Speaker) error {
		select {
		case <-ctx.Done():
		case <-s.ctx.Done():
		}
		return nil
	})
	c.changed(s)
	safe.Go("call clock", func() {
		tick := time.NewTicker(clockEvery)
		defer tick.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-tick.C:
				discovery.Get().Touch(s.Peer.ID)
				shell.Get().Redraw()
			}
		}
	})
}

func (c *Calls) lost(s *session) func(webrtc.PeerConnectionState) {
	return func(state webrtc.PeerConnectionState) {
		if c.state(s) != Idle {
			slog.Info("call link lost", "peer", s.Peer.Name, "state", state)
			c.end(s, ReasonDropped)
		}
	}
}

func (c *Calls) state(s *session) State {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.now != s {
		return Idle
	}
	return s.State
}

func (c *Calls) fail(s *session, step string, err error) {
	slog.Warn("call failed", "step", step, "peer", s.Peer.Name, "err", err)
	c.end(s, ReasonFailed)
}

func (c *Calls) end(s *session, reason Reason) {
	addr, id := s.addr, s.ID
	link := c.finish(s, reason)
	safe.Go("call end", func() {
		ctx, cancel := context.WithTimeout(context.Background(), postWait)
		defer cancel()
		if _, err := post(ctx, addr, "end", envelope{Call: id, Reason: reason}); err != nil {
			slog.Debug("call end not acknowledged", "err", err)
		}
		if link != nil {
			link.Close()
		}
	})
}

func (c *Calls) close(s *session, reason Reason) {
	if link := c.finish(s, reason); link != nil {
		link.Close()
	}
}

func (c *Calls) finish(s *session, reason Reason) *rtc.Link {
	c.mu.Lock()
	if c.now != s {
		c.mu.Unlock()
		return nil
	}
	c.now = nil
	link, hold, pics, peer := s.link, s.hold, s.pics, s.Peer.Name
	c.mu.Unlock()
	s.unhook()
	slog.Info("call ended", "peer", peer, "reason", reason)
	s.stop()
	pics.close()
	hold.Release()
	notice(peer, reason)
	c.publish(nil)
	shell.Get().Redraw()
	return link
}

func (c *Calls) changed(s *session) {
	c.mu.Lock()
	if c.now == s && s.hold == nil {
		c.mu.Unlock()
		h := shell.Get().Hold(&c.view)
		c.mu.Lock()
		s.hold = h
	}
	c.mu.Unlock()
	c.publish(s)
	shell.Get().Redraw()
}

func notice(peer string, reason Reason) {
	var body string
	switch reason {
	case ReasonDeclined:
		body = say.T("call.declined")
	case ReasonUnanswered:
		body = say.T("call.unanswered")
	case ReasonBusy:
		body = say.T("call.busy")
	case ReasonUnavailable:
		body = say.T("call.unavailable")
	case ReasonDropped:
		body = say.T("call.dropped")
	case ReasonFailed:
		body = say.T("call.failed")
	default:
		return
	}
	message.Get().Show(message.Message{Title: peer, Body: body, Tone: message.ToneInfo}, noticeFor)
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
