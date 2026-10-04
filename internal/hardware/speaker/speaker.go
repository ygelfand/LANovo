// Package speaker owns the amplifier and the playback stream.
//
// The amplifier is the internal codec's own, behind an enable line Lenovo's app used to drive. No
// WSA881x registers on this card, so the halves of mixer_paths_openq624_fep.xml that name one
// describe a board this is not; route has what is left, in the names this card has.
//
// Controls is there to answer the rest on the device rather than guess at it. Speaker protection
// is not among them: the feedback mux exists but the sense switch it feeds is a WSA881x control,
// so there is nothing on this card to protect the speaker with.
package speaker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/lib/alsa"
	"github.com/ygelfand/LANovo/internal/service"
)

// What the playback stream runs at. MultiMedia1 is card 0 device 0, playback and capture both.
const (
	Rate     = 48000
	Channels = 2
	Bits     = 16

	Card           = 0
	PlaybackDevice = 0

	period  = 960
	periods = 8
)

// FrameBytes is one frame across both channels.
const FrameBytes = Channels * Bits / 8

// Speaker is the output.
type Speaker struct {
	mu    sync.Mutex
	out   *alsa.Playback
	mixer *alsa.Mixer

	hwOnce     sync.Once
	hwOut      output
	hardVolume atomic.Bool
	amp        atomic.Pointer[component.Progress]

	// The queue the write loop drains a period at a time, under its own lock so filling a buffer
	// does not wait on the card being opened or closed.
	qmu     sync.Mutex
	pending []int16

	// level is the playback gain as float32 bits, applied as the queue is drained. Silent until
	// something sets it: the stored levels are restored at start-up, and a gain nobody chose is
	// not one to play at.
	level atomic.Uint32

	// Counts worth having when something sounds wrong. deaf is audio offered with no device,
	// splices are buffers the queue only part filled, underruns are buffers it did not reach at all
	// while something was playing.
	deaf      atomic.Uint64
	splices   atomic.Uint64
	underruns atomic.Uint64

	// fed carries whether the last buffer had anything in it, so the one after audio stops is still
	// written rather than the loop going quiet mid-tail.
	fed bool

	// The source placing audio by absolute frame, its scratch buffer, and how many frames have gone
	// to the card. Only the write loop touches the buffer and the count; the source itself can be
	// swapped from anywhere.
	srcMu   sync.Mutex
	src     Source
	srcBuf  []int16
	written atomic.Uint64

	// The voice resampler and its own lock, since it carries state across a reply's chunks and must
	// not be entered twice.
	voiceMu sync.Mutex
	voice   Resampler
	using   Resampling

	tap    atomic.Pointer[tapTo]
	tapBuf []int16

	mono []int16
	echo echo

	// loud is when the write loop last sent the card something audible, in unix nanoseconds.
	loud atomic.Int64
}

// audible is -60 dBFS, as a mean square per sample at int16 scale.
const audible = 1e-6 * 32768 * 32768

// soundingHold is how long after the last audible write the room still counts as hearing the speaker.
const soundingHold = 250 * time.Millisecond

// Sounding reports whether the speaker is making a sound, counting the tail after it goes quiet.
func (s *Speaker) Sounding() bool {
	at := s.loud.Load()
	return at != 0 && time.Since(time.Unix(0, at)) < soundingHold
}

type Tap interface{ Offer(mix []int16) }

type tapTo struct{ Tap }

// Offer runs on the write loop and must not block.
func (s *Speaker) SetTap(t Tap) {
	if t == nil {
		s.tap.Store(nil)
		return
	}
	s.tap.Store(&tapTo{t})
}

func init() {
	component.Register(component.Hardware, Get, component.Order(55),
		component.Supervise(service.Restart(2*time.Second, time.Minute)))
}

var (
	once   sync.Once
	shared *Speaker
)

func Get() *Speaker { once.Do(func() { shared = &Speaker{} }); return shared }

func (s *Speaker) Name() string { return "speaker" }

// Start opens the card and powers the amplifiers, so the write loop has somewhere to write.
func (s *Speaker) Start(context.Context) error {
	if s.open() {
		return nil
	}
	if err := s.Open(); err != nil {
		s.amp.Store(&component.Progress{Failed: true, Doing: err.Error()})
		return err
	}

	// After the stream, so the amplifiers come up to a running DAC rather than to a floating one.
	s.amp.Store(&component.Progress{Doing: "loading"})
	if err := s.Enable(true); err != nil {
		slog.Warn("enabling the amplifiers failed", "err", err)
		s.amp.Store(&component.Progress{Failed: true, Doing: err.Error()})
		return nil
	}
	s.amp.Store(&component.Progress{Done: true})
	return nil
}

// Enable powers the amplifiers. Separate from opening the stream, because a device that is not
// playing should not be holding them on: they are audible as hiss when idle.
func (s *Speaker) Enable(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.hw().power(s.out, on)
}

// Open routes the card and opens the stream. It does not enable the amplifiers and it does not
// play anything.
func (s *Speaker) Open() error {
	mixer, err := alsa.OpenMixer(Card)
	if err != nil {
		return fmt.Errorf("speaker: %w", err)
	}

	if err := apply(mixer, s.hw().route()); err != nil {
		mixer.Close()
		return err
	}

	out, err := alsa.OpenPlayback(Card, PlaybackDevice, alsa.Config{
		Rate:       Rate,
		Channels:   Channels,
		Format:     alsa.FormatS16LE,
		Bits:       Bits,
		PeriodSize: period,
		Periods:    periods,
	})
	if err != nil {
		mixer.Close()
		return fmt.Errorf("speaker: %w", err)
	}

	s.mu.Lock()
	s.out, s.mixer = out, mixer
	hard, err := s.hw().volume(mixer, decibels(s.Volume()))
	s.hardVolume.Store(hard)
	s.mu.Unlock()
	if err != nil {
		slog.Warn("setting the volume failed", "err", err)
	}
	return nil
}

// Close releases the card and drops the amplifiers.
func (s *Speaker) Close() error {
	s.mu.Lock()
	out, mixer := s.out, s.mixer
	s.out, s.mixer = nil, nil
	s.mu.Unlock()

	if out != nil {
		out.Close()
	}
	if mixer != nil {
		mixer.Close()
	}
	return s.Enable(false)
}

// Write plays interleaved samples.
func (s *Speaker) Write(samples []int16) error {
	s.mu.Lock()
	out := s.out
	s.mu.Unlock()

	if out == nil {
		return fmt.Errorf("speaker: not open")
	}

	buf := make([]byte, len(samples)*2)
	for i, v := range samples {
		buf[i*2] = byte(v)
		buf[i*2+1] = byte(v >> 8)
	}

	_, err := out.Write(buf)
	return err
}

// Controls is every mixer control whose name contains want, for finding what drives the
// amplifiers on a card whose XML does not say.
func Controls(want string) ([]string, error) {
	mixer, err := alsa.OpenMixer(Card)
	if err != nil {
		return nil, fmt.Errorf("speaker: %w", err)
	}
	defer mixer.Close()

	all, err := mixer.Controls()
	if err != nil {
		return nil, fmt.Errorf("speaker: listing controls: %w", err)
	}

	want = strings.ToLower(want)
	var found []string
	for _, c := range all {
		if want == "" || strings.Contains(strings.ToLower(c.Name), want) {
			found = append(found, c.Name)
		}
	}
	return found, nil
}

// Report logs what the card offers that looks like it belongs to the amplifiers, which is how the
// rest of the route gets found.
func Report() {
	for _, want := range []string{"spk", "wsa", "quat", "rx1", "boost"} {
		found, err := Controls(want)
		if err != nil {
			slog.Warn("listing mixer controls failed", "err", err)
			return
		}
		slog.Debug("mixer controls", "matching", want, "names", found)
	}
}
