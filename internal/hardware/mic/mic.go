package mic

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ygelfand/libcountertop/pkg/audio/aec"
	"github.com/ygelfand/libcountertop/pkg/audio/alsa"
	"github.com/ygelfand/libcountertop/pkg/audio/capture"
	"github.com/ygelfand/libcountertop/pkg/audio/level"
	"github.com/ygelfand/libcountertop/pkg/hook"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func init() {
	component.Register(sharedcomponent.Hardware, Get, sharedcomponent.Order(50),
		sharedcomponent.Supervise(service.Restart(2*time.Second, time.Minute)))
}

const (
	Rate     = 48000
	Channels = 2

	Card = 0

	period  = 960
	periods = 8
)

const VoiceSamples = period / Decimation

type Frame struct {
	Samples []int16

	At time.Time
}

type Mics struct {
	Frames hook.Hook[Frame]

	Speech hook.Hook[Frame]

	Heard hook.Hook[Frame]

	Echo hook.Hook[EchoFrame]

	*capture.History
	*level.Leveler

	hwOnce sync.Once
	hwIn   input

	mu      sync.Mutex
	capture *alsa.Capture
	mixer   *alsa.Mixer
	gain    int
	lift    int

	err error

	cancel   *aec.Canceller
	adapting atomic.Bool
	refDown  Downsampler
	refBuf   []int16

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
		lift := board.Current().MicLift()
		shared = &Mics{
			History: capture.NewHistory(Voice, time.Second),
			Leveler: level.New(level.Config{
				Rate:          Voice,
				Frame:         VoiceSamples,
				ReferenceGain: config.DefaultMicGain,
				Gain:          cfg.Gain + lift,
				Sensitivity:   cfg.Sensitivity,
				Leveling:      cfg.Leveling,
			}),
			lift:   lift,
			gain:   min(max(cfg.Gain, MinGain), MaxGain-lift),
			cancel: newCanceller(),
		}
		shared.pre = newPreprocessors(shared.cancel)
		shared.denoising.Store(cfg.Denoise)
		shared.adapting.Store(true)
	})
	return shared
}

func (m *Mics) Name() string { return "microphones" }

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
		_ = mixer.Close()
		return err
	}

	m.mu.Lock()
	gain := m.gain + m.lift
	m.mu.Unlock()

	if err := hw.gain(mixer, gain); err != nil {
		_ = mixer.Close()
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
		_ = mixer.Close()
		return fmt.Errorf("mic: %w", err)
	}

	m.mu.Lock()
	m.capture, m.mixer = capture, mixer
	m.mu.Unlock()

	slog.Info(
		"microphones",
		"device",
		hw.device(),
		"bits",
		hw.bits(),
		"rate",
		Rate,
		"channels",
		Channels,
		"period",
		period,
		"gain",
		gain,
	)
	return nil
}

func (m *Mics) Close() error {
	m.mu.Lock()
	capture, mixer := m.capture, m.mixer
	m.capture, m.mixer = nil, nil
	m.mu.Unlock()

	if capture != nil {
		_ = capture.Close()
	}
	if mixer != nil {
		_ = mixer.Close()
	}
	return nil
}

func (m *Mics) Startup() sharedcomponent.Progress {
	m.mu.Lock()
	open, err := m.capture != nil, m.err
	m.mu.Unlock()

	switch {
	case open:
		return sharedcomponent.Progress{
			Done:  true,
			Doing: fmt.Sprintf("%d kHz, %d channels", Rate/1000, Channels),
		}
	case err != nil:
		return sharedcomponent.Progress{Failed: true, Doing: err.Error()}
	}
	return sharedcomponent.Progress{Doing: "taking the microphones"}
}

func (m *Mics) Gain() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.gain
}

func (m *Mics) SetGain(gain int) error {
	gain = min(max(gain, MinGain), MaxGain-m.lift)

	m.mu.Lock()
	m.gain = gain
	mixer := m.mixer
	m.mu.Unlock()
	if m.Leveler != nil {
		m.SetInputGain(gain + m.lift)
	}

	if mixer == nil {
		return nil
	}
	return m.hw().gain(mixer, gain+m.lift)
}

func (m *Mics) Run(ctx context.Context) error {
	m.mu.Lock()
	capture := m.capture
	m.mu.Unlock()

	if capture == nil {
		return fmt.Errorf("mic: not open")
	}

	go func() {
		<-ctx.Done()
		_ = m.Close()
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

		speech := mono(left, right)

		m.SetPlayback(sounding)
		m.Process(speech)

		m.Add(speech)
		m.Speech.Emit(Frame{Samples: speech, At: at})
	}
}
