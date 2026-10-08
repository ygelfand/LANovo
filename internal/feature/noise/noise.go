package noise

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/audio/noise"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(36),
		sharedcomponent.Supervise(service.Restart(5*time.Second, time.Minute)))
}

const off = "Off"

const layers = 2

const (
	ahead = speaker.Rate / 2
	chunk = speaker.Rate / 20
)

const ducked = 0.2

type Machine struct {
	layers []*esphome.Select

	mu      sync.Mutex
	playing []string
	fill    noise.Fill
	down    bool
	gain    float32
}

var (
	once   sync.Once
	shared *Machine
)

func Get() *Machine {
	once.Do(func() {
		shared = &Machine{gain: 1}
		shared.build()
	})
	return shared
}

func (m *Machine) Name() string { return "noise machine" }

func (m *Machine) Entities() []esphome.Entity {
	out := make([]esphome.Entity, 0, len(m.layers))
	for _, sel := range m.layers {
		out = append(out, sel)
	}
	return out
}

func (m *Machine) Restore(config.Config) {
	for _, sel := range m.layers {
		sel.Set(off)
	}
}

func (m *Machine) build() {
	for i := 1; i <= layers; i++ {
		sel := &esphome.Select{
			Base: esphome.Base{
				ObjectID: fmt.Sprintf("noise_layer_%d", i),
				DeviceID: component.DevicePlayback,
				Name:     fmt.Sprintf("Sound machine layer %d", i),
				Icon:     "mdi:blur",
			},
			Options: append([]string{off}, noise.Names()...),
		}

		sel.OnCommand = func(chosen string) {
			if chosen != off && !noise.Has(chosen) {
				slog.Warn("no such sound", "layer", sel.ObjectID, "sound", chosen)
				return
			}
			sel.Set(chosen)
			m.sound()
		}
		m.layers = append(m.layers, sel)
	}
}

func (m *Machine) sound() {
	var chosen []string
	for _, sel := range m.layers {
		if s := sel.Get(); s != off && s != "" {
			chosen = append(chosen, s)
		}
	}

	if len(chosen) == 0 {
		m.Stop()
		return
	}

	fill := noise.Mix(speaker.Rate, chosen...)
	if fill == nil {
		slog.Warn("no such sounds", "sounds", chosen)
		return
	}

	m.mu.Lock()
	was := m.playing
	m.playing, m.fill = chosen, fill
	m.mu.Unlock()

	if len(was) > 0 && speaker.Sound().Backgrounds().Owns(m) {
		speaker.Get().Take()
	}

	speaker.Sound().Backgrounds().Took(m)
	volume.Get().Sounding(config.StreamMedia)
	slog.Info("sound machine", "playing", chosen)
}

func (m *Machine) Stop() {
	m.mu.Lock()
	was := m.playing
	m.playing, m.fill = nil, nil
	m.mu.Unlock()

	if len(was) == 0 {
		return
	}

	if speaker.Sound().Backgrounds().Owns(m) {
		speaker.Get().Drain()
	}
	speaker.Sound().Backgrounds().Gave(m)

	for _, sel := range m.layers {
		sel.Set(off)
	}
	slog.Info("sound machine", "stopped", was)
}

func (m *Machine) Playing() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.playing
}

func (m *Machine) Run(ctx context.Context) error {
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()

	mono := make([]float32, chunk)
	samples := make([]int16, len(mono)*speaker.Channels)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}

		m.mu.Lock()
		fill, down, gain := m.fill, m.down, m.gain
		m.mu.Unlock()

		if fill == nil || down || !speaker.Sound().Backgrounds().Owns(m) {
			continue
		}

		for speaker.Get().Queued() < ahead {
			fill(mono)
			for i, v := range mono {
				s := int16(v * gain * math.MaxInt16)
				samples[i*speaker.Channels] = s
				samples[i*speaker.Channels+1] = s
			}
			speaker.Get().Play(samples)
		}
	}
}
