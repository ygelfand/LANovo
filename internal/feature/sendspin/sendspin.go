package sendspin

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Sendspin/sendspin-go/pkg/protocol"
	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/lib/safe"
	"github.com/ygelfand/LANovo/internal/ui"
)

func init() {
	component.Register(component.Device, Get, component.Order(26))
}

// Player is the room's membership of a group, as Home Assistant sees it: a listening port and an
// advert saying the room is here.
type Player struct {
	enabled *esphome.Switch
	state   *esphome.TextSensor
	title   *esphome.TextSensor
	artist  *esphome.TextSensor

	out *out

	mu      sync.Mutex
	running context.CancelFunc
	wake    chan struct{}

	// joined is the server holding the room, set while one is connected. Home Assistant's transport
	// controls go to it, and there is nothing to send them to when it is nil.
	joined *session

	// playing is the group's own playback state, which is the only thing that tells a pause from a
	// track that ended: both leave this room silent.
	playing track
	paused  bool

	// artwork is the newest image the server sent, decoded once on arrival rather than on every
	// redraw.
	artwork *ui.Image

	// Where the track had reached when the server last said, and when that was. The server sends
	// progress now and then rather than continuously, so the bar runs from the clock between.
	at        time.Duration
	length    time.Duration
	serverAt  int64
	speed     int
	serverNow func() int64
}

// artworkSize is what the server is asked to scale album art to.
const artworkSize = 512

var (
	once   sync.Once
	shared *Player
)

func Get() *Player {
	once.Do(func() { shared = build() })
	return shared
}

func build() *Player {
	p := &Player{
		out:  newOut(speaker.Get()),
		wake: make(chan struct{}, 1),
	}
	p.out.pause = p.Pause

	p.enabled = &esphome.Switch{
		Base: esphome.Base{
			ObjectID: "sendspin",
			Name:     "Sendspin",
			Icon:     "mdi:speaker-multiple",
			Category: esphome.CategoryConfig,
			DeviceID: component.DevicePlayback,
		},
		OnCommand: func(on bool) { p.SetEnabled(on) },
	}

	p.state = &esphome.TextSensor{
		Base: esphome.Base{
			ObjectID: "sendspin_state",
			Name:     "Sendspin state",
			Icon:     "mdi:lan-connect",
			Category: esphome.CategoryDiagnostic,
			DeviceID: component.DevicePlayback,
		},
	}
	p.state.Set(stateOff)

	p.title = &esphome.TextSensor{
		Base: esphome.Base{
			ObjectID: "sendspin_title",
			Name:     "Now playing",
			Icon:     "mdi:music-note",
			DeviceID: component.DevicePlayback,
		},
	}
	p.artist = &esphome.TextSensor{
		Base: esphome.Base{
			ObjectID: "sendspin_artist",
			Name:     "Artist",
			Icon:     "mdi:account-music",
			DeviceID: component.DevicePlayback,
		},
	}
	clock.Get().Stepped.Listen(p.stepped)
	return p
}

func (p *Player) stepped(by time.Duration) {
	p.mu.Lock()
	joined := p.joined
	p.mu.Unlock()
	if joined == nil {
		return
	}
	slog.Info("the clock was stepped, starting the sendspin session again", "by", by.Round(time.Millisecond))
	joined.client.Close()
}

// What the state sensor says, from switched off to audible.
const (
	stateOff     = "off"
	stateWaiting = "waiting"
	stateJoined  = "joined"
	statePlaying = "playing"
)

func (p *Player) Name() string { return "sendspin" }

func (p *Player) Entities() []esphome.Entity {
	return []esphome.Entity{p.enabled, p.state, p.title, p.artist}
}

// Play, Pause and Stop implement media.Source: Home Assistant reaches for the speaker entity whoever
// started the audio, so when a group is playing these are what its buttons mean.
//
// Stop falls back to pause because a room that joined a group cannot end what the group is playing;
// leaving the group would silence this room and keep the rest going, which is not what stop means.
func (p *Player) Play()  { p.tell("play") }
func (p *Player) Pause() { p.tell("pause") }
func (p *Player) Stop()  { p.tell("stop", "pause") }

// Next and Previous move the whole group, not just this room: the server owns the queue, and a room
// asking to skip is asking on everyone's behalf.
func (p *Player) Next()     { p.tell("next") }
func (p *Player) Previous() { p.tell("previous") }

func (p *Player) tell(want ...string) {
	p.mu.Lock()
	s := p.joined
	p.mu.Unlock()

	if s == nil {
		return
	}

	// Off the caller's thread: this arrives on Home Assistant's read loop, and the send holds a lock
	// around a websocket write with no deadline on it. A server that stopped reading would take the
	// device's own connection down with it.
	safe.Go("sendspin command", func() { s.tell(want...) })
}

// Playing implements media.Source.
func (p *Player) Playing() (playing, paused bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state.Get() == statePlaying && !p.paused, p.paused
}

// Now implements media.Source: what the group is playing, for the entity and for the screen.
//
// Skipping is the server's to do and it does it for the whole house, so the controls offered are the
// ones a room may press on everyone else's behalf.
// Kind implements media.Source: a group the room has joined.
func (*Player) Kind() media.Kind { return media.FromGroup }

// Label implements media.Source. A server does not name itself in anything it sends, and the room's
// own name is this device's rather than the group's.
func (*Player) Label() string { return "" }

func (p *Player) Now() media.Now {
	playing, paused := p.Playing()

	p.mu.Lock()
	defer p.mu.Unlock()

	return media.Now{
		Playing: playing,
		Paused:  paused,
		Title:   p.playing.Title,
		Artist:  p.playing.Artist,
		Album:   p.playing.Album,
		Art:     p.artwork,
		Elapsed: p.reached(playing),
		Length:  p.length,
		Can:     media.CanPause | media.CanNext | media.CanPrevious,
	}
}

// reached is where the track is now: where the server last said, plus the time since, while the
// audio is going. Held to the length, since a server that stopped reporting would otherwise run the
// bar off the end.
func (p *Player) reached(playing bool) time.Duration {
	at := p.at
	if playing && p.serverNow != nil && p.speed > 0 {
		at += time.Duration((p.serverNow()-p.serverAt)*int64(p.speed)/1000) * time.Microsecond
	}
	if p.length > 0 && at > p.length {
		return p.length
	}
	return max(at, 0)
}

// moved takes the position the server reported.
func (p *Player) moved(progress protocol.ProgressState, stamp int64, serverNow func() int64) {
	if stamp == 0 {
		stamp = serverNow()
	}
	p.mu.Lock()
	p.at = time.Duration(progress.TrackProgress) * time.Millisecond
	p.length = time.Duration(progress.TrackDuration) * time.Millisecond
	p.serverAt, p.speed, p.serverNow = stamp, progress.PlaybackSpeed, serverNow
	at, length := p.at, p.length
	p.mu.Unlock()

	slog.Debug("sendspin progress", "at", at, "length", length, "speed", progress.PlaybackSpeed,
		"stamp_lead_ms", (stamp-serverNow())/1000)
	media.Get().Changed()
}

// holds says which session owns the room, and nil when none does. The media player follows it: the
// group answers Home Assistant's transport controls for as long as it is connected, whether or not
// audio happens to be arriving this second.
func (p *Player) holds(s *session) {
	p.mu.Lock()
	p.joined = s
	p.paused = false
	p.playing = track{}
	p.mu.Unlock()

	p.title.Set("")
	p.artist.Set("")

	if s == nil {
		media.Get().Ended(p)
		media.Get().Release(p)
	}
}

// setState publishes it and tells the media player, which reads Playing from it.
//
// Starting to play is also what claims the card, since the card follows the last thing to play.
func (p *Player) setState(state string) {
	p.state.Set(state)

	if state == statePlaying {
		media.Get().External(p)
		return
	}
	media.Get().Changed()
}

// grouped takes the group's playback state, which is what separates a pause from a track ending.
func (p *Player) grouped(state string) {
	p.mu.Lock()
	p.paused = state == "paused" || state == "stopped" && p.playing.Title != ""
	p.mu.Unlock()
	media.Get().Changed()
}

// track is what the room is playing, as far as the metadata role has said.
func (p *Player) track() track {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playing
}

// plays publishes the track. The empty one clears the sensors rather than leaving them naming
// something nobody can hear.
func (p *Player) plays(t track) {
	p.mu.Lock()
	if t != p.playing {
		p.at, p.length, p.serverAt, p.speed = 0, 0, 0, 0
	}
	p.playing = t
	p.mu.Unlock()

	p.title.Set(t.Title)
	p.artist.Set(t.Artist)
	media.Get().Changed()

	if !t.empty() {
		slog.Info("sendspin now playing", "title", t.Title, "artist", t.Artist, "album", t.Album)
	}
}

// drew keeps the newest album art.
func (p *Player) drew(art protocol.ArtworkChunk) {
	img, err := ui.Decode(art.Data)
	if err != nil {
		slog.Warn("sendspin artwork", "channel", art.Channel, "err", err)
		return
	}

	p.mu.Lock()
	p.artwork = img
	p.mu.Unlock()

	w, h := img.Size()
	slog.Debug("sendspin artwork", "channel", art.Channel, "size", fmt.Sprintf("%dx%d", w, h))
	media.Get().Changed()
}

// Artwork is the newest album art the server sent, and nil when there is none.
func (p *Player) Artwork() *ui.Image {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.artwork
}

// Restore puts the switch back where it was left. Listening waits for Run, once there is a network.
func (p *Player) Restore(c config.Config) { p.enabled.Set(c.Sendspin.Enabled) }

// Run holds the port open for as long as the switch is on.
func (p *Player) Run(ctx context.Context) error {
	defer p.stop()

	for {
		p.settle(ctx)

		select {
		case <-ctx.Done():
			return nil
		case <-p.wake:
		}
	}
}

// SetEnabled turns the room's part in whole-house audio on or off and remembers it.
//
// Exported because the screen changes this too, and both have to do the same three things: tell
// Home Assistant, write the file, and wake the loop. A second path that did two of them would leave
// the device listening with a file that says it is not.
func (p *Player) SetEnabled(on bool) {
	p.enabled.Set(on)

	if err := config.Set().Sendspin().Enabled(on); err != nil {
		slog.Error("saving a setting failed", "setting", p.enabled.ObjectID, "err", err)
		return
	}
	p.rethink()
}

// rethink wakes the loop without blocking. A second ask while one is pending is the same ask.
func (p *Player) rethink() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// settle makes what is running match what was asked for.
func (p *Player) settle(parent context.Context) {
	want := config.Get().Sendspin.Enabled

	p.mu.Lock()
	already := p.running != nil
	p.mu.Unlock()

	switch {
	case want && already, !want && !already:
		return
	case !want:
		p.stop()
		p.state.Set(stateOff)
		return
	}

	ctx, cancel := context.WithCancel(parent)
	p.mu.Lock()
	p.running = cancel
	p.mu.Unlock()

	// Nothing to open: this device's INPUT policy is ACCEPT, where the Echo's vendor firewall
	// dropped whatever it had not been told about.
	name := config.Get().Device.Name
	l := newListener(p.out, speaker.Sound().Backgrounds(), p)

	safe.Go("sendspin listen", func() {
		if err := l.serve(ctx, name); err != nil {
			slog.Error("sendspin listener stopped", "err", err)
		}
	})
	safe.Go("sendspin advertise", func() { advertise(ctx, name, Port) })

	p.state.Set(stateWaiting)
	slog.Info("sendspin waiting for a server", "name", name, "port", Port)
}

func (p *Player) stop() {
	p.mu.Lock()
	cancel := p.running
	p.running = nil
	p.mu.Unlock()

	if cancel == nil {
		return
	}
	cancel()
}
