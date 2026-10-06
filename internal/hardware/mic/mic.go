// Package mic owns the two digital microphones: one capture stream, held for the life of the
// process, with frames fanned out to whoever is listening.
//
// There is no tinycap or libasound on the device, so this drives the /dev/snd ioctls directly.
package mic

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"sync/atomic"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/lib/alsa"
	"github.com/ygelfand/LANovo/internal/service"
	"github.com/ygelfand/libcountertop/pkg/audio/aec"
	"github.com/ygelfand/libcountertop/pkg/hook"
)

func init() {
	component.Register(component.Hardware, Get, component.Order(50),
		component.Supervise(service.Restart(2*time.Second, time.Minute)))
}

// What the capture path runs at on every board: 48 kHz, two channels, in 960-frame periods.
const (
	Rate     = 48000
	Channels = 2

	Card = 0

	period  = 960
	periods = 8
)

// VoiceSamples is one period once it is at the voice rate, which is what the leveller and the
// wake word models work on.
const VoiceSamples = period / Decimation

// Frame is one period of interleaved samples, left and right alternating.
type Frame struct {
	Samples []int16

	// At is when the period was read, for anything lining audio up against something else.
	At time.Time
}

// Mics is one stream from both microphones.
type Mics struct {
	// Frames carries every period read, as captured: interleaved, both channels, at Rate.
	//
	// Listeners must not block: the reader is a device with a ring behind it, and a slow listener
	// becomes an overrun.
	Frames hook.Hook[Frame]

	// Speech carries the same period as mono at the voice rate, levelled. It is what the wake
	// word models and recognition listen to, and what the room level is measured on.
	Speech hook.Hook[Frame]

	Heard hook.Hook[Frame]

	Echo hook.Hook[EchoFrame]

	// history is the last second at the voice rate, for the words a turn missed by starting late.
	history history

	hwOnce sync.Once
	hwIn   input

	mu      sync.Mutex
	capture *alsa.Capture
	mixer   *alsa.Mixer
	gain    int

	// err is why the card could not be taken, for the boot screen to show.
	err error

	// leveling is the switch; the leveller itself belongs to the reader.
	leveling atomic.Bool
	leveler  *leveler

	cancel   *canceller
	adapting atomic.Bool

	pre          [2]*aec.Preprocessor
	denoising    atomic.Bool
	wasDenoising bool
}

var (
	once   sync.Once
	shared *Mics
)

func Get() *Mics {
	once.Do(func() {
		cfg := config.Get().Microphone
		shared = &Mics{gain: min(max(cfg.Gain, MinGain), MaxGain), leveler: newLeveler(), cancel: newCanceller()}
		shared.leveling.Store(cfg.Leveling)
		shared.pre = newPreprocessors(shared.cancel)
		shared.denoising.Store(cfg.Denoise)
		shared.adapting.Store(true)
	})
	return shared
}

func (m *Mics) Name() string { return "microphones" }

// Start routes the microphones and opens the stream.
// Start takes the card, remembering why if it cannot: a boot screen that waits forever on a card
// something else is holding tells nobody anything.
func (m *Mics) Start(ctx context.Context) error {
	err := m.open(ctx)

	m.mu.Lock()
	m.err = err
	m.mu.Unlock()

	return err
}

func (m *Mics) open(context.Context) error {
	mixer, err := alsa.OpenMixer(Card)
	if err != nil {
		return fmt.Errorf("mic: %w", err)
	}

	hw := m.hw()
	if err := hw.open(mixer); err != nil {
		mixer.Close()
		return err
	}

	m.mu.Lock()
	gain := m.gain
	m.mu.Unlock()

	if err := hw.gain(mixer, gain); err != nil {
		mixer.Close()
		return err
	}

	format := alsa.FormatS16LE
	if hw.bits() == 32 {
		format = alsa.FormatS32LE
	}
	capture, err := alsa.Open(Card, hw.device(), alsa.Config{
		Rate:       Rate,
		Channels:   Channels,
		Format:     format,
		Bits:       hw.bits(),
		PeriodSize: period,
		Periods:    periods,
	})
	if err != nil {
		mixer.Close()
		return fmt.Errorf("mic: %w", err)
	}

	m.mu.Lock()
	m.capture, m.mixer = capture, mixer
	m.mu.Unlock()

	slog.Info("microphones",
		"device", hw.device(), "bits", hw.bits(), "rate", Rate, "channels", Channels, "period", period, "gain", gain)
	return nil
}

// Close releases the card.
func (m *Mics) Close() error {
	m.mu.Lock()
	capture, mixer := m.capture, m.mixer
	m.capture, m.mixer = nil, nil
	m.mu.Unlock()

	if capture != nil {
		capture.Close()
	}
	if mixer != nil {
		mixer.Close()
	}
	return nil
}

// Startup is ready once the card is open and delivering.
func (m *Mics) Startup() component.Progress {
	m.mu.Lock()
	open, err := m.capture != nil, m.err
	m.mu.Unlock()

	switch {
	case open:
		return component.Progress{
			Done:  true,
			Doing: fmt.Sprintf("%d kHz, %d channels", Rate/1000, Channels),
		}
	case err != nil:
		// A device that cannot hear is still worth having, and the alternative is a boot screen
		// that waits for a card something else is holding and never finishes.
		return component.Progress{Failed: true, Doing: err.Error()}
	}
	return component.Progress{Doing: "taking the microphones"}
}

// Gain is the digital gain on both decimators.
func (m *Mics) Gain() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.gain
}

// SetGain changes it, on the card if it is open.
func (m *Mics) SetGain(gain int) error {
	gain = min(max(gain, MinGain), MaxGain)

	m.mu.Lock()
	m.gain = gain
	mixer := m.mixer
	m.mu.Unlock()
	if m.leveler != nil {
		m.leveler.atGain(gain)
	}

	if mixer == nil {
		return nil
	}
	return m.hw().gain(mixer, gain)
}

// Run reads periods until ctx is canceled.
func (m *Mics) Run(ctx context.Context) error {
	m.mu.Lock()
	capture := m.capture
	m.mu.Unlock()

	if capture == nil {
		return fmt.Errorf("mic: not open")
	}

	// A read blocks until the card has a period, so canceling closes the stream out from under
	// it rather than waiting.
	go func() {
		<-ctx.Done()
		m.Close()
	}()

	bits := m.hw().bits()
	frameBytes := Channels * bits / 8
	buf := make([]byte, period*frameBytes)
	var downL, downR Downsampler
	spk := speaker.Get()
	aligned := align{pipeline: int64(spk.Pipeline().Seconds() * Rate)}
	var read uint64
	var warned bool
	for {
		if ctx.Err() != nil {
			return nil
		}

		n, err := capture.Read(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("mic: reading: %w", err)
		}
		frames := uint64(n / frameBytes)
		read += frames
		pending, derr := capture.Delay()
		if derr != nil && !warned {
			warned = true
			slog.Warn("capture delay unavailable", "err", derr)
		}
		at := time.Now()
		samples := samplesOf(buf[:n], bits)
		m.Frames.Emit(Frame{Samples: samples, At: at})

		if playing, played, ok := spk.Playing(); ok && derr == nil {
			aligned.observe(read+uint64(max(pending, 0)), at, playing, played)
		}

		left, right := downL.Channel16k(samples, 0), downR.Channel16k(samples, 1)
		sounding := spk.Sounding()
		rawL, rawR := left, right
		ref := m.reference(&aligned, read-frames, int(frames), sounding)
		left, right, cancelled := m.clean(left, right, ref)
		if m.Echo.Listeners() > 0 {
			m.Echo.Emit(EchoFrame{
				Mic: [2][]int16{rawL, rawR}, Reference: ref, Out: [2][]int16{left, right},
				Cancelling: cancelled, Offset: aligned.offset, Aligned: aligned.locked,
			})
		}
		m.Heard.Emit(Frame{Samples: interleave(left, right), At: at})

		// Levelled at the voice rate rather than as captured: the leveller works on what
		// recognition hears, which is mono and decimated.
		speech := mono(left, right)

		// Measured whether or not it is applied, so the room level means the same either way.
		m.leveler.atPlayback(sounding)
		if m.leveling.Load() {
			m.leveler.apply(speech)
		} else {
			m.leveler.observe(speech)
		}

		m.history.add(speech)
		m.Speech.Emit(Frame{Samples: speech, At: at})
	}
}
