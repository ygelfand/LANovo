// Package a2dp is the device as a Bluetooth speaker.
//
// The pieces all existed and none of them were joined: internal/lib/bt decides what a phone's bytes
// mean, internal/lib/bt/pair decides a pairing, internal/lib/bt/sbc turns frames into samples, and
// internal/hardware/ble carries the line. This is the wiring, and nothing else — no protocol lives
// here, so a fault is in one of the parts rather than spread between them.
package a2dp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/ble"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/lib/bt"
	"github.com/ygelfand/LANovo/internal/lib/bt/avdtp"
	"github.com/ygelfand/LANovo/internal/lib/bt/avrcp"
	"github.com/ygelfand/LANovo/internal/lib/bt/l2cap"
	"github.com/ygelfand/LANovo/internal/lib/bt/pair"
	"github.com/ygelfand/LANovo/internal/lib/bt/sbc"
	"github.com/ygelfand/LANovo/internal/lib/bt/sdp"
	"github.com/ygelfand/LANovo/internal/service"
)

func init() {
	component.Register(component.Device, Get, component.Order(70),
		component.Supervise(service.Restart(5*time.Second, time.Minute)))
}

// bonds is where link keys live between boots, so a phone pairs once rather than every time.
const bonds = layout.StateDir + "/bonds"

// Sink is the device answering as a speaker.
type Sink struct {
	mu sync.Mutex

	sink  *bt.Sink
	pairs *pair.Manager

	// filter is the synthesis filterbank, which keeps its history between frames. One per stream:
	// restarting it per frame clicks at every frame boundary.
	filter sbc.Filter

	// resample carries the stream from whatever rate the phone chose to the one the card runs at.
	// Nil until a stream is configured, and it also keeps history across frames.
	resample *speaker.Rational
	drift    speaker.Drift

	enable *esphome.Switch

	// wake ends the current attempt when the setting changes, so the next one reads the new value.
	// Buffered and never blocked on: a toggle must not wait for the attempt to notice.
	wake chan struct{}

	// stop ends the attempt that is running.
	stop context.CancelFunc

	// send puts a PDU on the live link without waiting to be asked, and handle is the link to put
	// it on. Both are set while a phone is connected and nil between.
	send   func(handle uint16, pdu []byte) error
	handle uint16

	// track is what the phone last said it is playing, so a change can be told from a repeat, and
	// label what the phone calls itself.
	track avrcp.Track
	label string

	// queue is the now playing list as the browsing channel last gave it, and handles the artwork
	// each entry named, which is not in the listing and is asked for one at a time.
	queue   []avrcp.Item
	handles map[uint64]string

	// asks is the entries still to be asked about, at the one in flight, and counter the listing
	// they came from. A target refuses a uid against a counter that has moved on.
	//
	// An answer names no entry, so which one it belongs to is only what was asked last. A second
	// walk started over the top of one would put the answers on the wrong entries, so a listing
	// that arrives while one is running waits in later.
	asks    []uint64
	at      uint64
	counter uint16
	walked  time.Time

	later   []uint64
	waiting uint16

	// How many entries this pass asked about and how many named an image.
	asked int
	got   int

	// asking is when the sequence that fetches the list started. A second one overlapping it
	// interleaves, and the card takes whichever answer lands last.
	asking time.Time

	// answering is set while a frame is being answered, and held is what this end wanted to say
	// meanwhile. A channel that has just opened is not configured at the far end until our answer
	// lands, so anything sent before it is dropped.
	answering bool
	held      [][]byte

	// down is whether something else has the speaker, and gain the level to play at under a voice
	// turn. Both the arbiter's.
	down bool
	gain float32

	// streaming is whether audio is arriving. Ground truth for whether this device is playing,
	// where AVRCP only says what the phone believes.
	streaming bool

	// How the stream is going since the last time it said so.
	frames  int
	bad     int
	samples int
	why     error
	said    time.Time
}

var (
	once   sync.Once
	shared *Sink
)

func Get() *Sink { once.Do(func() { shared = build() }); return shared }

func build() *Sink {
	s := &Sink{
		wake: make(chan struct{}, 1),
		enable: &esphome.Switch{
			Base: esphome.Base{
				ObjectID: "bluetooth_speaker",
				Name:     "Bluetooth speaker",
				Icon:     "mdi:speaker-bluetooth",
				Category: esphome.CategoryConfig,
			},
		},
	}

	s.enable.Set(config.Get().Bluetooth.Speaker)
	s.enable.OnCommand = s.SetEnabled

	// A volume moved at this end is the phone's business: it holds a slider for this device and
	// registered to be told. Every change, however it was made, because the phone cannot tell a
	// button from Home Assistant and does not care.
	volume.Get().Changed.Listen(func(ch volume.Change) {
		if ch.Stream == config.StreamMedia {
			s.volumed(ch.Level)
		}
	})

	return s
}

func (s *Sink) Name() string { return "bluetooth speaker" }

func (s *Sink) Entities() []esphome.Entity { return []esphome.Entity{s.enable} }

// Enabled reports whether the device should be answering as a speaker.
func (s *Sink) Enabled() bool { return config.Get().Bluetooth.Speaker }

// SetEnabled turns the speaker on or off.
//
// Ending the current run is what makes it take: the supervisor starts another, and that one reads
// the setting as it now is. Turning it on while off ends a run that is only waiting; turning it off
// while serving ends one that is holding the line.
func (s *Sink) SetEnabled(on bool) {
	s.enable.Set(on)

	if err := config.Set().Bluetooth().Speaker(on); err != nil {
		slog.Error("saving the bluetooth speaker setting failed", "err", err)
		return
	}

	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// retry is how long to wait before looking again at whether the speaker can run.
//
// Not a backoff. What it is waiting for is usually something else letting go of the radio, which
// happens when a person turns the proxy off rather than on any schedule.
const retry = 10 * time.Second

// Run answers as a speaker for as long as it can, until ctx ends.
//
// A loop rather than one attempt. Returning ends the service for good — the supervisor treats a nil
// as finished — and the two things that stop this running, the setting being off and the radio
// being busy, both go away later without anything failing.
func (s *Sink) Run(ctx context.Context) error {
	ctx, stop := context.WithCancel(ctx)
	defer stop()

	// A change to the setting ends whatever the current attempt is doing, including a link that is
	// being carried, so the next pass reads the setting as it now is.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
				s.interrupt()
			}
		}
	}()

	for ctx.Err() == nil {
		if err := s.attempt(ctx); err != nil {
			slog.Warn("not answering as a speaker", "err", err)
		}

		select {
		case <-ctx.Done():
		case <-time.After(retry):
		}
	}
	return nil
}

// attempt is one go at being a speaker, for as long as it lasts.
func (s *Sink) attempt(parent context.Context) error {
	ctx, stop := context.WithCancel(parent)
	defer stop()

	s.mu.Lock()
	s.stop = stop
	s.mu.Unlock()

	if !s.Enabled() {
		return nil
	}

	radio := ble.Get()
	if !radio.Up() {
		return fmt.Errorf("a2dp: the radio is not up")
	}

	name := config.Get().Device.Name
	s.start(name)

	slog.Info("bluetooth speaker", "name", name, "class", "loudspeaker")
	return radio.Speaker(ctx, name, s)
}

// Sending takes the way to speak on the live link, and nil when it has gone.
func (s *Sink) Sending(send func(handle uint16, pdu []byte) error) {
	s.mu.Lock()
	s.send = send
	s.mu.Unlock()
}

// volumed tells the phone this device's volume moved, so its own slider follows.
//
// The phone is the controller for volume even though it is the source for audio, so this is the
// answer to a registration it made rather than a command. Nothing is sent for a level the phone
// itself just set.
func (s *Sink) volumed(level int) {
	s.tell(fmt.Sprintf("volume %d", level), func(sink *bt.Sink) ([]l2cap.Frame, error) {
		return sink.Volumed(level)
	})
}

// announce logs the track when it changes, the way a sendspin group does.
//
// On the track rather than on every change: the position moves constantly and a line each time is
// not a log. An empty one is worth saying too — it is the difference between a phone that sent
// nothing and one that was never asked.
func (s *Sink) announce(t avrcp.Track) {
	s.mu.Lock()
	was := s.track
	s.track = t
	s.mu.Unlock()

	// The handle arrives later than the rest: a phone registers the image after it has sent the
	// metadata, so it turns up on an answer where nothing else has moved.
	s.cover(t.Art)

	if t.Title == was.Title && t.Artist == was.Artist && t.Album == was.Album {
		return
	}

	slog.Info("bluetooth now playing", "title", t.Title, "artist", t.Artist, "album", t.Album,
		"duration", t.Duration, "art", t.Art)

	// A different track is a different place in the list, and often a different list. The handles
	// for what is in it come with that answer.
	s.list()
}

// interrupt ends the attempt that is running, if there is one.
func (s *Sink) interrupt() {
	s.mu.Lock()
	stop := s.stop
	s.mu.Unlock()

	if stop != nil {
		stop()
	}
}

// start builds the stack for one run and hangs the decoder off it.
func (s *Sink) start(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	keys, err := pair.Open(bonds)
	if err != nil {
		slog.Warn("pairings will not be remembered", "path", bonds, "err", err)
		s.pairs = pair.New(nil)
	} else {
		s.pairs = pair.New(keys)
	}

	s.sink = bt.NewSink(name, bt.MTU)
	s.filter = sbc.Filter{}
	s.queue, s.handles = nil, nil
	art.clear()

	// The phone's volume slider. Everything to receive it was already there — the remote control
	// accepts the command and answers it — but nothing was listening, so moving the slider changed
	// the number on the phone and nothing on this device.
	s.sink.Remote().Volume = func(percent int) {
		volume.Get().Set(config.StreamMedia, percent)
	}

	// The track moved, or the position, or the phone said what it is playing. The player pulls the
	// state when it redraws, so this only has to say that there is something new to pull.
	s.sink.Remote().Changed = func(state avrcp.State) {
		s.announce(state.Track)
		media.Get().Changed()
	}

	// The remote control channel is up: ask what is playing, for the channel the list comes over,
	// and what the phone serves images on. Until something asks, the phone says nothing.
	s.sink.Controls = func() { s.ask(); s.open(); s.look() }

	// Each channel this end asked for, as it comes up. Nothing can be sent on one before that, so
	// every sequence that needs a channel of its own continues from here.
	s.sink.Opened = func(psm uint16) {
		art.mu.Lock()
		images := art.psm
		art.mu.Unlock()

		switch {
		case psm == bt.BrowsePSM:
			s.list()
		case psm == l2cap.PSMSDP:
			s.look()
		case images != 0 && psm == images:
			s.Hello()
		}
	}

	s.sink.Image = s.imaged

	s.sink.Found = func(records []sdp.Record, more bool) {
		for _, r := range records {
			features, _ := r.Attribute(sdp.AttrFeatures)
			bits, _ := features.Uint()

			art, served := sdp.MorePSM(r, sdp.UUIDOBEX)
			browse, browsable := sdp.MorePSM(r, sdp.UUIDAVCTP)

			slog.Debug("bluetooth the far end serves",
				"classes", fmt.Sprintf("%#x", sdp.Classes(r)),
				"features", fmt.Sprintf("%#04x", bits),
				"cover_art_psm", fmt.Sprintf("%#04x", art), "cover_art", served,
				"browse_psm", fmt.Sprintf("%#04x", browse), "browsable", browsable)
		}
		if more {
			slog.Debug("bluetooth the far end held back more records than fitted")
		}
		s.looked(records)
	}

	s.sink.Listed = s.listed

	s.sink.Started = func(e *avdtp.Endpoint) {

		// A fresh filterbank per stream, not per frame: its history belongs to the configuration
		// the phone just chose.
		s.mu.Lock()
		s.filter = sbc.Filter{}
		s.mu.Unlock()

		// Whatever was queued belonged to something else. A stream starting on top of it plays
		// the tail of the last thing first.
		speaker.Get().Drain()

		// Taking the background stands the others down, and is what stops a group and a phone both
		// playing into the room at once.
		speaker.Sound().Backgrounds().Took(s)

		s.mu.Lock()
		s.streaming = true

		// The tally counts from the last time it spoke. Left alone, the first report after a stream
		// starts spans the silence before it and reads as starving.
		s.said = time.Now()
		s.frames, s.bad, s.samples, s.why = 0, 0, 0, nil
		s.mu.Unlock()

		// Audio starting is what claims the card, not the phone connecting.
		s.attach()

		// A new stream is new content, so neither the last one's track nor its list describes it.
		if remote := s.remote(); remote != nil {
			remote.Cleared()
		}
		s.holds(nil)
		s.ask()

		codec, ok := e.Codec()
		if !ok {
			slog.Info("bluetooth audio started")
			return
		}

		rate, _ := codec.Rate()

		// The card is fixed at one rate: it is opened once and shared, so whatever the phone chose
		// has to be carried to that. 44.1 is mandatory for a sink and is what a phone picks, and
		// played as 48 it is a semitone and a half sharp with the card starving besides.
		//
		// interleave always produces two channels, mono included.
		s.mu.Lock()
		s.resample = speaker.NewSkewable(rate, speaker.Rate, 2)
		s.drift.Reset()
		s.mu.Unlock()

		up, down := s.resample.Ratio()
		slog.Info("bluetooth audio started",
			"rate", rate, "mode", codec.Channels, "bitpool", codec.MaxBitpool,
			"resampled", fmt.Sprintf("%d:%d", up, down))
	}

	s.sink.Stopped = func(e *avdtp.Endpoint) {
		slog.Info("bluetooth audio stopped")

		s.mu.Lock()
		s.streaming = false
		s.mu.Unlock()

		speaker.Sound().Backgrounds().Gave(s)

		// The next stream may be at another rate, and in any case does not continue this one.
		s.mu.Lock()
		s.resample = nil
		s.mu.Unlock()

		// Not closed: the speaker belongs to the device rather than to this stream, and everything
		// else that makes a sound is still using it.
		speaker.Get().Drain()
	}

	s.sink.Frame = s.play
}

// play decodes one SBC frame and puts it on the speaker.
//
// Dropped rather than retried on a bad frame. Audio is a stream: a frame that will not decode is a
// click, and stopping over it is a song that ends.
func (s *Sink) play(frame []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// The window runs from the first frame, not from when the stream was announced. Audio follows
	// the announcement by something over a tenth of a second, and counting that silence against
	// the card's rate reads as starving every time a stream starts.
	if s.frames == 0 {
		s.said = time.Now()
	}

	s.frames++
	dumping.frame(frame)

	got, err := sbc.Unpack(frame)
	if err != nil {
		s.bad++
		s.why = err
		s.tally()
		return
	}

	audio, err := s.filter.Synthesize(got)
	if err != nil {
		s.bad++
		s.why = err
		s.tally()
		return
	}

	// Read directly: this already holds the lock, and sync.Mutex does not nest.
	if s.down {
		return
	}

	out := interleave(audio)
	decoded := out

	if s.resample != nil {
		out = s.resample.Run(out)
	}
	dumping.write(decoded, out)
	if s.gain != 0 {
		speaker.Scale(out, s.gain)
	}

	// Counted after resampling, because what the card is being handed is the number that says
	// whether it is being fed at the rate it runs at.
	s.samples += len(out) / 2
	s.tally()

	speaker.Get().Play(out)

	if s.resample != nil {
		s.resample.Skew(s.drift.Observe(speaker.Get().Queued(), time.Now()))
	}
}

// counted is how often a stream that is going wrong says so.
//
// Only when it is going wrong. A line every few seconds for the whole of a song is not a log, and
// nothing about a stream that decodes cleanly at the card's rate needs saying while it happens —
// the summary when it stops covers that.
const counted = 5 * time.Second

// adrift is how far the rate may sit from the card's before it is worth a line. A percent of 48000
// is 480 samples a second, which is well inside the jitter of counting over a few seconds and well
// outside anything that would be audible as drift.
const adrift = speaker.Rate / 100

// tally complains if the stream is not keeping up, now and then.
func (s *Sink) tally() {
	if time.Since(s.said) < counted {
		return
	}
	since := time.Since(s.said)

	// Samples per second against the card's rate is the thing to read: short means the stream is
	// starving, long means it is arriving faster than the card takes it.
	rate := int(float64(s.samples) / since.Seconds())

	if s.bad > 0 || rate < speaker.Rate-adrift || rate > speaker.Rate+adrift {
		slog.Warn("bluetooth audio is not keeping up", "frames", s.frames, "bad", s.bad,
			"samples_per_second", rate, "card", speaker.Rate, "last", s.why)
	}

	s.said = time.Now()
	s.frames, s.bad, s.samples, s.why = 0, 0, 0, nil
}

// interleave lays the channels out the way the sound card wants them, and makes a mono stream
// stereo by sending it to both ears rather than one.
func interleave(audio sbc.Samples) []int16 {
	if len(audio) == 0 {
		return nil
	}

	if len(audio) == 1 {
		out := make([]int16, 0, len(audio[0])*2)
		for _, v := range audio[0] {
			out = append(out, v, v)
		}
		return out
	}

	out := make([]int16, 0, len(audio[0])*len(audio))
	for i := range audio[0] {
		for ch := range audio {
			out = append(out, audio[ch][i])
		}
	}
	return out
}

// Event answers an HCI event, which is a pairing decision and nothing else.
//
// Logged, because a connection that ends on its own says nothing about why otherwise, and the
// events are a handful per connection. Advertising reports are not: those are the other radio and
// arrive by the hundred.
func (s *Sink) Event(e pair.Event) ([]pair.Command, error) {
	s.mu.Lock()
	pairs := s.pairs
	s.mu.Unlock()

	if pairs == nil {
		return nil, nil
	}

	said, err := pairs.Handle(e)
	if err != nil {
		slog.Warn("bluetooth pairing", "code", fmt.Sprintf("%#02x", e.Code), "err", err)
		return nil, err
	}
	return append(said, s.named(e)...), nil
}

// named asks a phone what it is called, and takes the answer. The card says who is playing, and a
// link carries an address until something asks.
func (s *Sink) named(e pair.Event) []pair.Command {
	switch e.Code {
	case pair.EventConnectionComplete:
		addr, ok := pair.Connected(e.Params)
		if !ok {
			return nil
		}
		return []pair.Command{pair.RemoteName(addr)}

	case pair.EventRemoteName:
		_, name, err := pair.ParseRemoteName(e.Params)
		if err != nil {
			slog.Warn("bluetooth name", "err", err)
			return nil
		}

		s.mu.Lock()
		s.label = name
		s.mu.Unlock()

		slog.Info("bluetooth connected", "name", name)
		media.Get().Changed()
	}
	return nil
}

// Label is what the phone calls itself, and empty until it has said.
func (s *Sink) Label() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.label
}

// brief is a payload as hex, cut short. Audio is the only thing here that runs long and it is not
// worth reading a packet of.
func brief(b []byte) string {
	const most = 40

	if len(b) <= most {
		return fmt.Sprintf("% x", b)
	}
	return fmt.Sprintf("% x ... (%d bytes)", b[:most], len(b))
}

// Data answers one L2CAP PDU off a link.
func (s *Sink) Data(handle uint16, pdu []byte) ([][]byte, error) {
	s.mu.Lock()
	sink := s.sink

	// Which link to speak on, for anything this device says of its own accord. Taken from data
	// rather than from the connection event because it is the same handle and this is where it is
	// certainly in use.
	s.handle = handle
	s.mu.Unlock()

	if sink == nil {
		return nil, nil
	}

	f, err := l2cap.ParseFrame(pdu)
	if err != nil {
		return nil, err
	}

	// Everything but the audio, which is the only thing here that arrives by the hundred.
	said := tracing() && !sink.Media(f.CID)
	if said {
		slog.Info("bluetooth said", "cid", fmt.Sprintf("%#04x", f.CID), "pdu", brief(f.Payload))
	}

	s.mu.Lock()
	s.answering = true
	s.mu.Unlock()

	out, err := sink.Receive(f)

	s.mu.Lock()
	held := s.held
	s.answering, s.held = false, nil
	s.mu.Unlock()

	if err != nil {
		slog.Warn("bluetooth data", "cid", fmt.Sprintf("%#04x", f.CID),
			"asked", brief(f.Payload), "err", err)
		return nil, err
	}

	if said {
		for _, r := range out {
			slog.Info("bluetooth answered", "cid", fmt.Sprintf("%#04x", r.CID), "pdu", brief(r.Payload))
		}
	}

	replies := make([][]byte, 0, len(out)+len(held))
	for _, r := range out {
		replies = append(replies, r.Marshal())
	}
	return append(replies, held...), nil
}

// Gone lets go of what the link was using, so the next phone starts from nothing.
//
// The reader is shared, so every link's disconnection arrives here. Only this sink's own is acted
// on.
func (s *Sink) Gone(handle uint16) {
	s.mu.Lock()
	ours, held := s.handle != 0 && s.handle == handle, s.handle
	s.mu.Unlock()

	slog.Info("bluetooth link ended", "handle", handle, "held", held, "ours", ours)

	if !ours {
		return
	}

	// Only if it is still ours. A phone that leaves does not take the card from whatever claimed it
	// since.
	s.detach()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sink != nil {
		s.sink.Forget()
	}
	s.filter = sbc.Filter{}
	s.handle = 0
	s.queue, s.handles, s.label = nil, nil, ""
}
