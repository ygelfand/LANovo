package visuals

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/ui/visual"
	"github.com/ygelfand/libcountertop/pkg/audio/analysis"
)

const (
	drainEvery = 20 * time.Millisecond
	replyFloor = 0.02

	LabelMost = 32
)

type Visuals struct {
	mic, spk *stereo
	heard    loudest
	ring     *analysis.Ring

	mu    sync.Mutex
	holds int
	stop  context.CancelFunc
	began time.Time
	last  time.Time

	listen func() (<-chan []int16, func())
	muted  func() bool
	tap    func(speaker.Tap)

	kind   *esphome.Select
	chosen visual.Kind
	label  *esphome.Text
	named  string
}

func init() {
	component.Register(component.Device, Get, component.Order(60))
}

func (v *Visuals) Name() string { return "visuals" }

func (v *Visuals) Entities() []esphome.Entity { return []esphome.Entity{v.kind, v.label} }

func (v *Visuals) Restore(cfg config.Config) {
	k := visual.Kind(cfg.Visual.Kind)
	if !slices.Contains(visual.Built(), k) {
		k = visual.Kind(config.DefaultVisual)
	}
	v.mu.Lock()
	v.chosen = k
	v.named = cfg.Visual.Label
	v.mu.Unlock()
	v.kind.Set(k.Label())
	v.label.Set(cfg.Visual.Label)
	visual.SetSeed(cfg.Visual.Seed)
}

func (v *Visuals) SetMaxFPS(fps int) {
	if !slices.Contains(config.MaxFPSSteps, fps) {
		fps = config.DefaultMaxFPS
	}
	if err := config.Set().Visual().MaxFPS(fps); err != nil {
		slog.Error("saving the visual frame cap failed", "err", err)
	}
}

func (v *Visuals) SetSeed(n int) {
	n = max(0, min(n, visual.SeedMost))
	visual.SetSeed(n)
	if err := config.Set().Visual().Seed(n); err != nil {
		slog.Error("saving the visual seed failed", "err", err)
	}
}

func (v *Visuals) SetLabel(s string) {
	if r := []rune(strings.TrimSpace(s)); len(r) > LabelMost {
		s = string(r[:LabelMost])
	} else {
		s = string(r)
	}
	v.mu.Lock()
	v.named = s
	v.mu.Unlock()
	v.label.Set(s)
	if err := config.Set().Visual().Label(s); err != nil {
		slog.Error("saving a setting failed", "setting", v.label.ObjectID, "err", err)
	}
}

// Kind is the visual the device is set to draw.
func (v *Visuals) Kind() visual.Kind {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.chosen
}

// SetKind changes the visual and remembers it.
func (v *Visuals) SetKind(k visual.Kind) {
	v.mu.Lock()
	v.chosen = k
	v.mu.Unlock()
	v.kind.Set(k.Label())
	if err := config.Set().Visual().Kind(string(k)); err != nil {
		slog.Error("saving a setting failed", "setting", v.kind.ObjectID, "err", err)
	}
}

func (v *Visuals) build() {
	v.chosen = visual.Kind(config.DefaultVisual)
	v.kind = &esphome.Select{
		Base: esphome.Base{ObjectID: "visual", Name: "Visual", Icon: "mdi:waveform",
			Category: esphome.CategoryConfig, DeviceID: component.DeviceScreen},
		Options: config.Labels(visual.Built()),
	}
	v.kind.OnCommand = func(label string) {
		if k, ok := config.ByLabel(visual.Built(), label); ok {
			v.SetKind(k)
		}
	}
	v.named = config.DefaultLabel
	v.label = &esphome.Text{
		Base: esphome.Base{ObjectID: "visual_label", Name: "Visual label", Icon: "mdi:format-text",
			Category: esphome.CategoryConfig, DeviceID: component.DeviceScreen},
		MaxLength: LabelMost,
	}
	v.label.OnCommand = v.SetLabel
}

var (
	once   sync.Once
	shared *Visuals
)

func Get() *Visuals {
	once.Do(func() {
		shared = newVisuals(
			func() (<-chan []int16, func()) { return mic.Get().ListenStereo("visuals") },
			func() bool { return privacy.Get().MicMuted() },
			speaker.Get().SetTap,
		)
	})
	return shared
}

func newVisuals(listen func() (<-chan []int16, func()), muted func() bool, tap func(speaker.Tap)) *Visuals {
	v := &Visuals{
		mic:    newStereo(mic.Voice, liftGain(config.Get().Microphone.VisualizerLift)),
		spk:    newStereo(speaker.Rate, 1),
		ring:   analysis.NewRing(speaker.Rate * speaker.Channels / 5),
		listen: listen, muted: muted, tap: tap,
	}
	v.build()
	return v
}

func (v *Visuals) Hold() (release func()) {
	v.mu.Lock()
	v.holds++
	if v.holds == 1 {
		ctx, stop := context.WithCancel(context.Background())
		v.stop, v.last = stop, time.Now()
		v.mic.reset()
		v.spk.reset()
		v.heard.reset()
		v.tap(v.ring)
		go v.drain(ctx)
		go v.hear(ctx)
	}
	v.mu.Unlock()

	var done sync.Once
	return func() { done.Do(v.release) }
}

func (v *Visuals) release() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.holds--; v.holds > 0 {
		return
	}
	v.tap(nil)
	v.stop()
	v.stop = nil
}

func (v *Visuals) SetLift(db int) { v.mic.setGain(liftGain(min(max(db, 0), LiftMax))) }

func (v *Visuals) Loudest() float64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.heard.most
}

func (v *Visuals) Input() visual.Input {
	v.mu.Lock()
	now := time.Now()
	if v.began.IsZero() {
		v.began, v.last = now, now
	}
	x := visual.Input{Now: now.Sub(v.began), Dt: now.Sub(v.last), Label: v.named}
	v.last = now
	v.mu.Unlock()

	x.Mic, x.Speaker = v.mic.mix.Latest(), v.spk.mix.Latest()
	x.MicLeft, x.MicRight = v.mic.left.Latest(), v.mic.right.Latest()
	x.SpeakerLeft, x.SpeakerRight = v.spk.left.Latest(), v.spk.right.Latest()
	x.Replying = x.Speaker.Level > replyFloor && x.Speaker.Level >= x.Mic.Level
	return x
}

func (v *Visuals) drain(ctx context.Context) {
	t := time.NewTicker(drainEvery)
	defer t.Stop()
	var buf []int16
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		buf = v.ring.Drain(buf)
		v.spk.write(buf)
	}
}

func (v *Visuals) hear(ctx context.Context) {
	frames, stop := v.listen()
	defer stop()
	var silent []int16
	for {
		select {
		case <-ctx.Done():
			return
		case f, ok := <-frames:
			if !ok {
				return
			}
			if v.muted() {
				if cap(silent) < len(f) {
					silent = make([]int16, len(f))
				}
				f = silent[:len(f)]
			}
			v.mu.Lock()
			v.heard.hear(f)
			v.mu.Unlock()
			v.mic.write(f)
		}
	}
}
