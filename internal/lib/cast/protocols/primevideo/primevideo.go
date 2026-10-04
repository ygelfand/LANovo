package primevideo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/lib/cast"
	"github.com/ygelfand/LANovo/internal/lib/cast/playback"
	"github.com/ygelfand/LANovo/internal/lib/surface"
)

const (
	Name = "primevideo"
	NS   = "urn:x-cast:com.amazon.primevideo.cast"
)

type Protocol struct {
	env     cast.Env
	auth    string
	kept    cast.Kept
	persist func() bool

	mu        sync.Mutex
	saved     Registration
	access    string
	locale    string
	envelopes map[string]Envelope
	cur       *title
	media     cast.Media
}

type Registration struct {
	Device      string `json:"device,omitempty"`
	Actor       string `json:"actor,omitempty"`
	Marketplace string `json:"marketplace,omitempty"`
	Refresh     string `json:"refresh,omitempty"`
}

type Envelope struct {
	Envelope    string `json:"envelope"`
	Correlation string `json:"correlationId,omitempty"`
}

type incoming struct {
	Type        string    `json:"type"`
	Device      string    `json:"deviceId"`
	Marketplace string    `json:"marketplaceId"`
	Actor       string    `json:"actorId"`
	LinkCode    string    `json:"preAuthorizedLinkCode"`
	Content     string    `json:"contentId"`
	Envelope    *Envelope `json:"playbackEnvelope"`
	Settings    struct {
		Locale string `json:"locale"`
	} `json:"settings"`
}

type failure struct {
	Code     string `json:"code"`
	Internal string `json:"internalName"`
	Message  string `json:"message"`
	Fatal    bool   `json:"isFatal"`
}

type answer struct {
	Type  string   `json:"type"`
	Error *failure `json:"error,omitempty"`
}

func init() {
	cast.Define(Name, func(env cast.Env) cast.Protocol { return New(env) })
}

func New(env cast.Env) *Protocol {
	p := &Protocol{
		env:       env,
		auth:      AuthHost,
		persist:   func() bool { return config.Get().Cast.Prime.Persist },
		locale:    "en_US",
		envelopes: map[string]Envelope{},
	}
	if env.Keep != nil {
		p.kept = env.Keep(Name)
	}
	if !p.persist() {
		p.discard()
	}
	return p
}

func (p *Protocol) discard() {
	if p.kept == nil {
		return
	}
	if err := p.kept.Clear(); err != nil {
		slog.Warn("clearing the saved prime video registration", "err", err)
	}
}

func (p *Protocol) Unsave() { p.discard() }

func (p *Protocol) ForgetRegistration() {
	p.mu.Lock()
	p.saved, p.access = Registration{}, ""
	p.mu.Unlock()
	p.discard()
}

func (p *Protocol) Registered() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.saved.Refresh != "" && p.saved.Actor != ""
}

func (p *Protocol) Started(app cast.App) {
	p.forget()
	if p.env.Output != nil {
		p.env.Output.Attend(&playback.Session{Label: app.Name, Logo: app.Icon, Pictured: true})
	}
}

func (p *Protocol) Ended(cast.App) {
	if t := p.playing(); t != nil {
		go p.stop(t)
	}
	p.forget()
	if p.env.Output != nil {
		p.env.Output.Attend(nil)
	}
}

func (p *Protocol) forget() {
	var saved Registration
	if p.kept != nil && p.persist != nil && p.persist() {
		p.kept.Load(&saved)
	}
	p.mu.Lock()
	p.saved, p.access, p.envelopes = saved, "", map[string]Envelope{}
	p.mu.Unlock()
}

func (p *Protocol) Name() string            { return Name }
func (p *Protocol) Namespaces() []string    { return []string{NS} }
func (p *Protocol) Run(ctx context.Context) {}

func (p *Protocol) Receive(app *cast.Application, m cast.Message) ([]cast.Message, error) {
	if m.Namespace != NS {
		return nil, cast.ErrUnspoken
	}
	var in incoming
	if err := json.Unmarshal([]byte(m.Payload), &in); err != nil {
		return nil, fmt.Errorf("prime video: %w", err)
	}
	slog.Debug("prime video message", "type", in.Type, "device", in.Device, "marketplace", in.Marketplace, "content", in.Content)

	switch in.Type {
	case "AmIRegistered":
		p.mu.Lock()
		if d := strings.TrimSpace(in.Device); d != "" {
			p.saved.Device = d
		}
		registered, device := p.saved.Refresh != "" && p.saved.Actor != "", p.saved.Device
		p.mu.Unlock()
		a := answer{Type: "AmIRegisteredResponse"}
		if !registered {
			a.Error = &failure{Code: "NotRegistered", Internal: "NotRegistered", Message: "deviceId " + device + " is not registered"}
		}
		return p.reply(m, a)

	case "Register":
		p.mu.Lock()
		if v := strings.TrimSpace(in.Marketplace); v != "" {
			p.saved.Marketplace = v
		}
		if v := strings.TrimSpace(in.Device); v != "" {
			p.saved.Device = v
		}
		if v := strings.TrimSpace(in.Actor); v != "" {
			p.saved.Actor = v
		}
		device, actor := p.saved.Device, p.saved.Actor
		p.mu.Unlock()
		if in.LinkCode == "" || actor == "" || p.env.Send == nil {
			return p.reply(m, answer{Type: "RegisterResponse"})
		}
		go p.register(m, in.LinkCode, device, actor)
		return nil, nil

	case "ApplySettings":
		p.mu.Lock()
		if l := in.Settings.Locale; l != "" {
			p.locale = strings.ReplaceAll(l, "-", "_")
		}
		if d := strings.TrimSpace(in.Device); d != "" {
			p.saved.Device = d
		}
		p.mu.Unlock()
		return p.reply(m, answer{Type: "ApplySettingsResponse"})

	case "Preload":
		if in.Content != "" && in.Envelope != nil && in.Envelope.Envelope != "" {
			p.mu.Lock()
			p.envelopes[in.Content] = *in.Envelope
			p.mu.Unlock()
		}
		return p.reply(m, answer{Type: "PreloadResponse"})
	}
	return nil, cast.ErrUnspoken
}

func (p *Protocol) register(m cast.Message, code, device, actor string) {
	ctx := context.Background()
	refresh, err := Register(ctx, p.env.HTTP, p.auth, code, device)
	if err == nil {
		var access string
		access, err = ActorToken(ctx, p.env.HTTP, p.auth, actor, refresh)
		if err == nil {
			p.mu.Lock()
			p.saved.Refresh, p.access = refresh, access
			saved := p.saved
			p.mu.Unlock()
			if p.kept != nil && p.persist != nil && p.persist() {
				if err := p.kept.Save(saved); err != nil {
					slog.Warn("saving the prime video registration", "err", err)
				}
			}
			slog.Info("prime video registered", "device", device, "marketplace", saved.Marketplace)
		}
	}
	if err != nil {
		slog.Warn("prime video registration", "err", err)
	}
	if out, err := p.reply(m, answer{Type: "RegisterResponse"}); err == nil {
		for _, o := range out {
			p.env.Send(o)
		}
	}
}

func (p *Protocol) reply(m cast.Message, a answer) ([]cast.Message, error) {
	body, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return []cast.Message{cast.Reply(m, NS, string(body))}, nil
}

type loadRequest struct {
	RequestID int     `json:"requestId"`
	Current   float64 `json:"currentTime"`
	Media     struct {
		ContentID   string          `json:"contentId"`
		ContentType string          `json:"contentType"`
		StreamType  string          `json:"streamType"`
		Custom      json.RawMessage `json:"customData"`
	} `json:"media"`
	Custom struct {
		Device      string    `json:"deviceId"`
		Marketplace string    `json:"marketplaceId"`
		Envelope    *Envelope `json:"playbackEnvelope"`
		Tracks      struct {
			Audio struct {
				Language string `json:"language"`
			} `json:"AUDIO"`
		} `json:"initialTracks"`
	} `json:"customData"`
}

func (p *Protocol) Load(app *cast.Application, m cast.Message) ([]cast.Message, error) {
	var req loadRequest
	if err := json.Unmarshal([]byte(m.Payload), &req); err != nil {
		return []cast.Message{cast.Reply(m, cast.NSMedia, cast.LoadFailed(0))}, fmt.Errorf("prime video: load: %w", err)
	}
	go p.resolve(m, req)
	return nil, nil
}

func (p *Protocol) resolve(m cast.Message, req loadRequest) {
	t, err := p.resolved(req)
	if err != nil {
		slog.Warn("prime video load", "title", req.Media.ContentID, "err", err)
		if p.env.Output != nil {
			p.env.Output.Failed(err)
		}
		if p.env.Send != nil {
			p.env.Send(cast.Reply(m, cast.NSMedia, cast.LoadFailed(req.RequestID)))
		}
		return
	}
	p.mu.Lock()
	old := p.cur
	p.cur, p.media = t, cast.Media{
		ContentID: req.Media.ContentID, ContentType: req.Media.ContentType, StreamType: cast.StreamBuffered,
		Duration: t.length.Seconds(), CustomData: req.Media.Custom,
		Metadata: cast.Metadata{Title: "Prime Video"},
	}
	p.mu.Unlock()
	if old != nil {
		p.env.Output.Handoff(old)
		old.close()
	}
	p.env.Output.Play(t)
	p.publish(t, cast.StatePlaying, &m, req.RequestID)
	go p.describe(t)
}

func (p *Protocol) describe(t *title) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	name, series, err := Catalog(ctx, p.env.HTTP, t.acct, t.id)
	if err != nil || name == "" && series == "" {
		slog.Debug("prime video catalog", "title", t.id, "err", err)
		return
	}
	p.mu.Lock()
	if p.cur != t {
		p.mu.Unlock()
		return
	}
	t.name, t.series = name, series
	p.media.Metadata.Title, p.media.Metadata.Subtitle = first(name, "Prime Video"), series
	p.mu.Unlock()
	slog.Debug("prime video catalog", "title", t.id, "name", name, "series", series)
	p.env.Output.Changed(t)
	state := cast.StatePlaying
	if p.paused(t) {
		state = cast.StatePaused
	}
	p.publish(t, state, nil, 0)
}

func (p *Protocol) resolved(req loadRequest) (*title, error) {
	id := strings.TrimSpace(req.Media.ContentID)
	if id == "" {
		return nil, errors.New("no content id")
	}
	p.mu.Lock()
	a := Account{Token: p.access, Device: p.saved.Device, Marketplace: p.saved.Marketplace, Locale: p.locale}
	env, preloaded := p.envelopes[id]
	p.mu.Unlock()
	if v := strings.TrimSpace(req.Custom.Device); v != "" {
		a.Device = v
	}
	if v := strings.TrimSpace(req.Custom.Marketplace); v != "" {
		a.Marketplace = v
	}
	if a.Token == "" {
		return nil, errors.New("not registered")
	}
	if p.env.Output == nil {
		return nil, errors.New("no output")
	}
	if !preloaded {
		if req.Custom.Envelope == nil || req.Custom.Envelope.Envelope == "" {
			return nil, errors.New("no playback envelope")
		}
		env = *req.Custom.Envelope
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if env.Correlation != "" {
		if fresh, err := RefreshEnvelope(ctx, p.env.HTTP, a, id, env.Correlation); err != nil {
			slog.Debug("prime video envelope kept", "err", err)
		} else {
			env = fresh
		}
	}
	res, err := VodResources(ctx, p.env.HTTP, a, p.screen(), id, env.Envelope)
	if err != nil {
		return nil, err
	}
	target := res.URLs[0].URL
	for _, u := range res.URLs {
		if u.ID == res.Default {
			target = u.URL
		}
	}
	data, err := Manifest(ctx, p.env.HTTP, target)
	if err != nil {
		return nil, err
	}
	lang := req.Custom.Tracks.Audio.Language
	if lang == "" {
		lang = p.locale
	}
	parts, err := Pick(data, target, p.screen().Tallest, lang)
	if err != nil {
		return nil, err
	}
	from := time.Duration(req.Current * float64(time.Second))
	if at, ok := Bumper(parts); ok && config.Get().Cast.Prime.SkipIntro && from < at {
		slog.Info("prime video skipping intro", "title", id, "to", at)
		from = at
	}
	t, err := openTitle(ctx, p, title{
		proto: p, id: id, acct: a, envelope: env.Envelope, handoff: res.Handoff,
		parts: parts, length: Length(data), from: from,
	})
	if err != nil {
		return nil, err
	}
	logTitle(t)
	return t, nil
}

func (p *Protocol) screen() Screen {
	var t playback.Target
	if p.env.Video != nil {
		t = p.env.Video()
	}
	return Screen{Width: t.Width, Height: t.Height, Tallest: t.Tallest, Codecs: []string{"H264"}, MaxResolution: resolution(t.Tallest)}
}

func resolution(tallest int) string {
	switch {
	case tallest <= 576:
		return "SD"
	case tallest <= 720:
		return "720p"
	case tallest <= 1080:
		return "1080p"
	}
	return "UHD"
}

func (p *Protocol) surface() *surface.Client {
	if p.env.Surface == nil {
		return nil
	}
	return p.env.Surface()
}

func (p *Protocol) current(t *title) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cur == t
}

func (p *Protocol) publish(t *title, state string, answer *cast.Message, request int) {
	if p.env.Publish == nil || !p.current(t) {
		return
	}
	p.mu.Lock()
	m := p.media
	p.mu.Unlock()
	p.env.Publish(control{p}, &cast.Published{Media: m, State: state, Commands: 15, Answer: answer, Request: request})
}

func (p *Protocol) paused(t *title) bool {
	_, paused := p.env.Output.Position(t)
	return paused
}

func (p *Protocol) play(t *title) {
	if !p.current(t) {
		return
	}
	p.env.Output.Resume(t)
	p.publish(t, cast.StatePlaying, nil, 0)
}

func (p *Protocol) pause(t *title) {
	if !p.current(t) {
		return
	}
	p.env.Output.Pause(t)
	p.publish(t, cast.StatePaused, nil, 0)
}

func (p *Protocol) buffering(t *title, w bool) {
	switch {
	case w:
		p.publish(t, cast.StateBuffering, nil, 0)
	case p.paused(t):
		p.publish(t, cast.StatePaused, nil, 0)
	default:
		p.publish(t, cast.StatePlaying, nil, 0)
	}
}

func (p *Protocol) stop(t *title) {
	p.mu.Lock()
	if p.cur != t || t == nil {
		p.mu.Unlock()
		return
	}
	p.cur = nil
	p.mu.Unlock()
	p.env.Output.Stop(t)
	t.close()
	if p.env.Publish != nil {
		p.env.Publish(control{p}, nil)
	}
}

func (p *Protocol) seek(t *title, to time.Duration) {
	if !p.current(t) {
		return
	}
	held := p.paused(t)
	p.publish(t, cast.StateBuffering, nil, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	next := *t
	next.from = to
	nt, err := openTitle(ctx, p, next)
	if err != nil {
		slog.Warn("prime video seek", "to", to, "err", err)
		p.publish(t, cast.StatePlaying, nil, 0)
		return
	}
	p.mu.Lock()
	if p.cur != t {
		p.mu.Unlock()
		nt.close()
		return
	}
	p.cur = nt
	p.mu.Unlock()
	p.env.Output.Handoff(t)
	t.close()
	p.env.Output.Play(nt)
	state := cast.StatePlaying
	if held {
		p.env.Output.Pause(nt)
		state = cast.StatePaused
	}
	p.publish(nt, state, nil, 0)
}

func (p *Protocol) playing() *title {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cur
}

type control struct{ p *Protocol }

func (c control) Load(cast.Media, time.Duration, bool) error {
	return errors.New("prime video: load through the sender")
}

func (c control) Play() error {
	if t := c.p.playing(); t != nil {
		go c.p.play(t)
	}
	return nil
}

func (c control) Pause() error {
	if t := c.p.playing(); t != nil {
		go c.p.pause(t)
	}
	return nil
}

func (c control) Stop() error {
	if t := c.p.playing(); t != nil {
		go c.p.stop(t)
	}
	return nil
}

func (c control) Seek(to time.Duration) error {
	if t := c.p.playing(); t != nil {
		go c.p.seek(t, to)
	}
	return nil
}

func (c control) Elapsed() time.Duration {
	t := c.p.playing()
	if t == nil || c.p.env.Output == nil {
		return 0
	}
	at, _ := c.p.env.Output.Position(t)
	return at
}
