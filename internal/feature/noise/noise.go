// Package noise is the sound machine: generated sound that runs until somebody stops it.
//
// It plays as a producer on the speaker's arbiter rather than as an errand, because that is what it
// is — a long sound that stands aside for a track or a reply and picks up again afterwards. Nothing
// is stored or streamed: the sounds are synthesized a chunk at a time, so the queue is kept just
// full enough to play without a gap.
package noise

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/lib/noise"
	"github.com/ygelfand/LANovo/internal/service"
)

func init() {
	component.Register(component.Device, Get, component.Order(36),
		component.Supervise(service.Restart(5*time.Second, time.Minute)))
}

// off is the option that stops a layer, which is the one they all start on. A sound machine that
// came back by itself after a restart would be a device that started making noise in an empty room.
const off = "Off"

// layers is how many sounds can run at once. Two is what makes it worth having: rain on its own is
// rain, and rain over a fan is a room.
const layers = 2

// ahead is how many frames to keep queued, and chunk how many are generated at a time. Enough that
// a late wake-up does not leave a gap, little enough that stopping is not heard to lag.
const (
	ahead = speaker.Rate / 2
	chunk = speaker.Rate / 20
)

// ducked is how far it drops under a voice turn.
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

// Restore is not restoring anything: the layers are what somebody asked for, and a device that came
// back from an update hissing in a dark room is a fault to anyone in it.
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

// sound plays whatever the layers add up to, and stops when they add up to nothing. One generator
// for all of them: mixing at the source keeps the sum inside full scale, where two producers each
// filling the queue would clip against each other.
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

	// Changing a layer while another is running replaces the generator, so what is queued is the
	// old mix and would be heard before the new one. Only when the queue is ours: standing aside
	// means it belongs to whatever took over.
	if len(was) > 0 && speaker.Sound().Backgrounds().Owns(m) {
		speaker.Get().Take()
	}

	speaker.Sound().Backgrounds().Took(m)
	volume.Get().Sounding(config.StreamMedia)
	slog.Info("sound machine", "playing", chosen)
}

// Stop ends whatever is running and lets go of the background.
func (m *Machine) Stop() {
	m.mu.Lock()
	was := m.playing
	m.playing, m.fill = nil, nil
	m.mu.Unlock()

	if len(was) == 0 {
		return
	}

	// Only what is ours. Turning the layers off while a track has the speaker should stop the
	// sound machine, not silence the track.
	if speaker.Sound().Backgrounds().Owns(m) {
		speaker.Get().Drain()
	}
	speaker.Sound().Backgrounds().Gave(m)

	for _, sel := range m.layers {
		sel.Set(off)
	}
	slog.Info("sound machine", "stopped", was)
}

// Playing is what the layers add up to, empty when there is nothing.
func (m *Machine) Playing() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.playing
}

// Run keeps the queue topped up while a sound is running and this is the one being heard.
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

		// Both channels get the same samples: one enclosure, one pair of drivers.
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
