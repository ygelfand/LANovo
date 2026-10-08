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

// MultiMedia1 is card 0 device 0, playback and capture both.
const (
	Rate     = 48000
	Channels = 2
	Bits     = 16

	Card           = 0
	PlaybackDevice = 0

	period  = 960
	periods = 8
)

const FrameBytes = Channels * Bits / 8

type Speaker struct {
	mu    sync.Mutex
	out   *alsa.Playback
	mixer *alsa.Mixer

	hwOnce     sync.Once
	hwOut      output
	hardVolume atomic.Bool
	amp        atomic.Pointer[component.Progress]

	qmu     sync.Mutex
	pending []int16

	level atomic.Uint32

	deaf      atomic.Uint64
	splices   atomic.Uint64
	underruns atomic.Uint64

	fed bool

	srcMu   sync.Mutex
	src     Source
	srcBuf  []int16
	written atomic.Uint64

	voiceMu sync.Mutex
	voice   Resampler
	using   Resampling

	tap    atomic.Pointer[tapTo]
	tapBuf []int16

	mono []int16
	echo echo

	loud atomic.Int64
}

const audible = 1e-6 * 32768 * 32768

const soundingHold = 250 * time.Millisecond

func (s *Speaker) Sounding() bool {
	at := s.loud.Load()
	return at != 0 && time.Since(time.Unix(0, at)) < soundingHold
}

type Tap interface{ Offer(mix []int16) }

type tapTo struct{ Tap }

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

func (s *Speaker) Start(context.Context) error {
	if s.open() {
		return nil
	}
	if err := s.Open(); err != nil {
		s.amp.Store(&component.Progress{Failed: true, Doing: err.Error()})
		return err
	}

	s.amp.Store(&component.Progress{Doing: "loading"})
	if err := s.Enable(true); err != nil {
		slog.Warn("enabling the amplifiers failed", "err", err)
		s.amp.Store(&component.Progress{Failed: true, Doing: err.Error()})
		return nil
	}
	s.amp.Store(&component.Progress{Done: true})
	return nil
}

// The amplifiers hiss audibly when enabled and idle.
func (s *Speaker) Enable(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.hw().power(s.out, on)
}

func (s *Speaker) Open() error {
	mixer, err := alsa.OpenMixer(Card)
	if err != nil {
		return fmt.Errorf("speaker: %w", err)
	}

	if err := apply(mixer, s.hw().route()); err != nil {
		_ = mixer.Close()
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
		_ = mixer.Close()
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

func (s *Speaker) Close() error {
	s.mu.Lock()
	out, mixer := s.out, s.mixer
	s.out, s.mixer = nil, nil
	s.mu.Unlock()

	if out != nil {
		_ = out.Close()
	}
	if mixer != nil {
		_ = mixer.Close()
	}
	return s.Enable(false)
}

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
