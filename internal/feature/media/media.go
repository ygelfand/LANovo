// Package media is the speaker as Home Assistant sees it: something to send sound to.
//
// Home Assistant converts whatever it is playing to WAV before sending it, so there is no decoder
// here. What arrives is samples, and they go to the card through the same queue everything else uses.
package media

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/service"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
)

func init() {
	component.Register(component.Device, Get, component.Order(35),
		component.Supervise(service.Restart(5*time.Second, time.Minute)))
}

// Player is the media player entity and whatever it is playing.
type Player struct {
	mp *esphome.MediaPlayer

	mu sync.Mutex

	// cur is what the player was last asked for. Held as a pointer so a fetch that has been
	// abandoned can tell it is no longer the current one.
	cur *track

	playing bool

	// speaking is set while a reply or an announcement is sounding, which Home Assistant is told is
	// the player playing: to the room the device is making noise either way.
	speaking atomic.Bool

	// held is the screen this put up, so stopping takes away that and nothing else, and sounded is
	// when something was last playing.
	held    *shell.Hold
	sounded time.Time
	stalled time.Time
	live    Source
	begun   Source

	// What the arbiter has said. down is standing aside for another producer, kept is the audio put
	// aside when that happened, and gain is the duck applied to what is written next.
	down bool
	kept []int16
	gain float32
}

type track struct {
	// stop abandons the fetch. A download that has been silenced should not arrive and play itself.
	stop context.CancelFunc

	// fetching is the download still in flight. Nothing is queued until it lands, so without this
	// the drain check calls the track finished before it has started.
	fetching bool
}

var (
	once   sync.Once
	shared *Player
)

func Get() *Player {
	once.Do(func() {
		shared = &Player{gain: 1}
		shared.build()
		onRail()
	})
	return shared
}

func (p *Player) Name() string { return "media player" }

func (p *Player) Entities() []esphome.Entity { return []esphome.Entity{p.mp} }

// Restore puts the level back and says what the player is doing, which at startup is nothing.
func (p *Player) Restore(config.Config) { p.refresh() }

func (p *Player) build() {
	p.mp = &esphome.MediaPlayer{
		// On the device itself rather than the playback page: this is the thing people reach for,
		// and it should be where they look first.
		Base: esphome.Base{ObjectID: "speaker", Name: "Speaker", Icon: "mdi:speaker"},
		Features: esphome.MediaPlayerFeatureVolumeSet |
			esphome.MediaPlayerFeatureVolumeStep |
			esphome.MediaPlayerFeatureVolumeMute |
			esphome.MediaPlayerFeaturePlayMedia |
			esphome.MediaPlayerFeatureBrowseMedia |
			esphome.MediaPlayerFeaturePlay |
			esphome.MediaPlayerFeatureStop |
			esphome.MediaPlayerFeatureAnnounce,
		SupportedFormats: Formats,
	}

	p.mp.OnCommand = p.command
}

// Run keeps the entity's state honest: what is queued drains on its own, and nothing else would
// notice that the last of it had gone.
func (p *Player) Run(ctx context.Context) error {
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}

		// Standing aside empties the queue without the track being over: what was in it is held
		// until whatever took the background gives it back.
		p.mu.Lock()
		done := p.playing && !p.down && len(p.kept) == 0 &&
			(p.cur == nil || !p.cur.fetching) && speaker.Get().Queued() == 0
		if done {
			p.cur, p.playing = nil, false
		}
		p.mu.Unlock()

		if done {
			speaker.Sound().Backgrounds().Gave(p)
			p.refresh()
		}
		p.follow()
		p.tick()
	}
}

func (p *Player) tick() {
	if !Showing() {
		return
	}
	now := p.Now()
	if !now.Playing || now.Length <= 0 {
		return
	}
	if Page().(*screen).moved(now.Elapsed) {
		shell.Get().Redraw()
	}
}

// Began claims the card for a source that is playing. The kind holding the live session is carrying
// on, a next track, a resume, a restarted stream; any other kind is taking over, which puts its
// player up and stops whatever held the card.
func (p *Player) Began(s Source) {
	var replaced Source
	p.mu.Lock()
	if p.live == nil || p.live.Kind() != s.Kind() {
		p.begun = s
		if held := p.source(); held != nil && held.Kind() != s.Kind() {
			replaced = held
		}
	}
	p.live = s
	p.mu.Unlock()
	p.External(s)
	if replaced != nil {
		slog.Info("the player was replaced", "was", fmt.Sprintf("%T", replaced), "by", fmt.Sprintf("%T", s))
		replaced.Stop()
	}
}

// Ended says a source's session is over, so the next time it begins is a start again.
func (p *Player) Ended(s Source) {
	p.mu.Lock()
	if p.live != nil && p.live.Kind() == s.Kind() {
		p.live = nil
	}
	p.mu.Unlock()
}

// follow puts the player up when a source has begun and takes it away afterwards.
func (p *Player) follow() {
	now := p.Now()
	playing := now.Playing || now.Hold
	src := p.source()

	idle := !now.Playing && (now.Paused || now.Hold)
	p.mu.Lock()
	begun := p.begun
	p.begun = nil
	if playing {
		p.sounded = time.Now()
	}
	switch {
	case !idle:
		p.stalled = time.Time{}
	case p.stalled.IsZero():
		p.stalled = time.Now()
	}
	stalled := time.Since(p.stalled)
	held, quiet := p.held, time.Since(p.sounded)
	if !playing && quiet > gap && begun == nil {
		p.live = nil
	}
	p.mu.Unlock()

	switch {
	case begun != nil && begun == src:
		slog.Info("the player opened", "by", fmt.Sprintf("%T", src))
		p.Open()

	case !playing && held.Held() && quiet > gap:
		slog.Info("the player is done", "quiet", quiet.Round(time.Second))
		held.Keep(false)
	}

	if limit := config.Get().Idle.Media.After(); idle && src != nil && limit > 0 && stalled > limit {
		slog.Info("the player was left idle", "for", stalled.Round(time.Second), "by", fmt.Sprintf("%T", src))
		p.mu.Lock()
		p.stalled = time.Time{}
		p.mu.Unlock()
		src.Stop()
	}
}

func (p *Player) SetIdle(v config.Delay) {
	if err := config.Set().Idle().Media(v); err != nil {
		slog.Warn("saving the media idle timeout", "err", err)
	}
}

// gap is how long nothing may be playing before the card goes.
const gap = 5 * time.Second

// Open shows the player for whatever is playing: the source's own if it has one, the card if not.
func (p *Player) Open() {
	if o, ok := p.source().(Opener); ok && o.Open() {
		p.mu.Lock()
		card := p.held
		p.held = nil
		p.mu.Unlock()
		card.Release()
		return
	}
	p.mu.Lock()
	held := p.held
	p.mu.Unlock()
	if held.Held() {
		return
	}
	hold := shell.Get().Hold(Page())
	p.mu.Lock()
	p.held = hold
	p.mu.Unlock()
}

// Pause quietens whatever is playing without giving up its place, which is what a voice turn wants
// from it. A source that cannot pause is stopped instead: silence is the point.
func (p *Player) Pause() {
	if s := p.source(); s != nil {
		s.Pause()
		return
	}
	p.Stop()
}

func (p *Player) command(c esphome.MediaCommand) {
	if c.HasVolume {
		volume.Get().Set(config.StreamMedia, int(math.Round(float64(c.Volume)*100)))
		p.refresh()
	}

	if c.HasMediaURL && c.MediaURL != "" {
		p.play(c.MediaURL, c.Announcement)
	}

	if c.HasCommand && c.Command == esphome.MediaPlayerStop {
		p.Ended(homeAssistant{})
	}

	if !c.HasCommand {
		return
	}

	// Transport goes to whoever is actually playing. Volume does not: that is the card's, however
	// the sound got there.
	if s := p.source(); s != nil {
		switch c.Command {
		case esphome.MediaPlayerPlay:
			s.Play()
			return
		case esphome.MediaPlayerPause:
			s.Pause()
			return
		case esphome.MediaPlayerStop:
			s.Stop()
			return
		}
	}

	switch c.Command {
	case esphome.MediaPlayerVolumeUp:
		p.adjust(volume.Step)
	case esphome.MediaPlayerVolumeDown:
		p.adjust(-volume.Step)
	case esphome.MediaPlayerMute:
		p.Mute(true)
	case esphome.MediaPlayerUnmute:
		p.Mute(false)
	case esphome.MediaPlayerStop:
		p.Stop()
	}
}

// play fetches a url and queues it. Home Assistant serves it already converted, at the card's rate
// for music and at the pipeline's for an announcement.
func (p *Player) play(url string, announcement bool) {
	p.Stop()

	ctx, stop := context.WithCancel(context.Background())
	t := &track{stop: stop, fetching: true}

	p.mu.Lock()
	p.cur, p.playing = t, true
	p.mu.Unlock()

	// Playing is what claims the card, for this queue as for every other source.
	if announcement {
		p.External(homeAssistant{})
	} else {
		p.Began(homeAssistant{})
	}

	// Before anything is queued: taking the background is what stands the others down, and they put
	// their audio aside rather than leaving it in the queue under this one.
	speaker.Sound().Backgrounds().Took(p)
	p.refresh()

	safe.Go("media fetch", func() {
		defer stop()
		defer func() {
			p.mu.Lock()
			t.fetching = false
			p.mu.Unlock()
		}()

		samples, err := Fetch(ctx, url)
		if err != nil {
			slog.Error("fetching media failed", "url", url, "err", err)

			p.mu.Lock()
			if p.cur == t {
				p.playing = false
			}
			p.mu.Unlock()
			speaker.Sound().Backgrounds().Gave(p)
			p.refresh()
			return
		}
		if ctx.Err() != nil {
			return
		}

		// Something took the background while this was downloading, so the queue is theirs now.
		if !speaker.Sound().Backgrounds().Owns(p) {
			return
		}

		speaker.Scale(samples, p.ducking())

		if announcement {
			// Mono at the pipeline's rate, so it goes through the resampler like a spoken reply,
			// with a tail so the last word is not clipped by the filter running out.
			speaker.Get().PlayVoice(samples)
			speaker.Get().PlayVoice(make([]int16, speaker.VoiceRate*Tail/1000))
			return
		}
		speaker.Get().Play(samples)
	})
}

// Stop abandons whatever is playing and throws away what has not been heard.
func (p *Player) Stop() {
	p.mu.Lock()
	cur := p.cur
	p.cur, p.playing = nil, false
	p.kept = nil
	p.mu.Unlock()

	if cur != nil {
		cur.stop()
	}

	// Only what is ours. Stopping a track that has already stood aside should not empty the queue
	// of whatever took the speaker from it.
	if speaker.Sound().Backgrounds().Owns(p) {
		speaker.Get().Drain()
	}

	// After draining: leaving the background says whatever was waiting behind this may be heard,
	// and it should not come back to find this track's remainder still queued.
	speaker.Sound().Backgrounds().Gave(p)
	p.refresh()
}

// Playing reports whether anything is sounding, which a voice turn asks before ducking.
// Sounding marks a reply or an announcement as playing, and puts back whatever the player was doing
// once it ends.
func (p *Player) Sounding(on bool) {
	p.speaking.Store(on)
	p.refresh()
}

func (p *Player) Playing() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.playing
}

func (p *Player) adjust(by int) {
	v := volume.Get()
	v.Set(config.StreamMedia, v.Level(config.StreamMedia)+by)
	p.refresh()
}

// Mute silences the card whoever is playing. Exported because a group the room has joined sets it
// from the other end, and Home Assistant should show that the same way as a local mute.
func (p *Player) Mute(on bool) {
	p.mp.SetMuted(on)
	p.refresh()
}

func (p *Player) refresh() {
	p.mp.SetVolume(float32(volume.Get().Level(config.StreamMedia)) / 100)

	if p.mp.Muted() {
		speaker.Get().SetVolume(0)
	} else {
		volume.Get().Sounding(config.StreamMedia)
	}

	now := p.Now()
	switch {
	case now.Playing || p.speaking.Load():
		p.mp.SetState(esphome.MediaPlayerPlaying)
	case now.Paused:
		p.mp.SetState(esphome.MediaPlayerPaused)
	default:
		p.mp.SetState(esphome.MediaPlayerIdle)
	}

	// The screen reads the same thing the entity does, so it is stale until something paints it
	// again: a track changing under it arrives here and nowhere else.
	if Showing() {
		shell.Get().Redraw()
	}
}
