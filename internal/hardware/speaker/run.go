package speaker

import (
	"context"
	"encoding/binary"
	"log/slog"
	"math"
	"time"

	"github.com/ygelfand/libcountertop/pkg/audio/alsa"
	"github.com/ygelfand/libcountertop/pkg/audio/mix"
	"github.com/ygelfand/libcountertop/pkg/audio/volume"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

func (s *Speaker) Run(ctx context.Context) error {
	buf := make([]byte, period*Channels*Bits/8)
	var warned bool

	for {
		if ctx.Err() != nil {
			return nil
		}

		s.fill(buf)

		out := s.device()
		if out == nil {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(period * time.Second / Rate):
			}
			continue
		}

		if err := mix.Send(ctx, out, buf, &s.xruns); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		end := s.written.Add(period)
		queued, err := out.Delay()
		if err != nil && !warned {
			warned = true
			slog.Warn("playback delay unavailable", "err", err)
		}
		s.echo.commit(end-period, s.mono, end-uint64(max(queued, 0)), time.Now(), err == nil)
	}
}

func (s *Speaker) Attach(src mix.Source) { s.bus.Attach(src) }

func (s *Speaker) Written() uint64 { return s.written.Load() }

func (s *Speaker) fill(buf []byte) {
	want := period * Channels
	if cap(s.sum) < want {
		s.sum = make([]int32, want)
	}
	mixed := s.sum[:want]
	queued, rendered := s.bus.Next(s.written.Load(), mixed)

	switch {
	case queued > 0 && queued < want:
		s.splices.Add(1)
	case queued == 0 && s.fed:
		s.underruns.Add(1)
	}
	s.fed = queued > 0 || rendered

	tap := s.tap.Load()
	if tap != nil && cap(s.tapBuf) < want {
		s.tapBuf = make([]int16, want)
	}

	if cap(s.mono) < period {
		s.mono = make([]int16, period)
	}
	s.mono = s.mono[:period]

	gain := s.Volume()
	out := gain
	if s.hardVolume.Load() {
		out = 1
	}
	stereo := s.hw().stereo()
	var energy float64
	var sums [Channels]int32
	for f := range period {
		var both int32
		for c := range Channels {
			i := f*Channels + c
			sum := mixed[i]
			if tap != nil {
				s.tapBuf[i] = mix.Clamp(int32(float32(sum) * gain))
			}
			sums[c] = sum
			both += sum
		}

		s.mono[f] = mix.Clamp(int32(float32(both) / Channels * gain))
		for c := range Channels {
			v := mix.Clamp(int32(float32(both) / Channels * out))
			if stereo {
				v = mix.Clamp(int32(float32(sums[c]) * out))
			}
			binary.LittleEndian.PutUint16(buf[(f*Channels+c)*2:], uint16(v))
			heard := float64(v) * float64(gain/out)
			if out == 0 {
				heard = 0
			}
			energy += heard * heard
		}
	}
	if energy/float64(want) > audible {
		s.loud.Store(time.Now().UnixNano())
	}
	if tap != nil {
		tap.Offer(s.tapBuf[:want])
	}
}

func (s *Speaker) open() bool { return s.device() != nil }

func (s *Speaker) device() *alsa.Playback {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.out
}

func (s *Speaker) SetVolume(gain float32) {
	gain = max(0, min(gain, 1))
	s.level.Store(math.Float32bits(gain))

	s.mu.Lock()
	hard, err := s.hw().volume(s.mixer, decibels(gain))
	s.hardVolume.Store(hard)
	s.mu.Unlock()
	if err != nil {
		slog.Warn("setting the volume failed", "err", err)
	}
}

func (s *Speaker) Volume() float32 {
	return math.Float32frombits(s.level.Load())
}

func (s *Speaker) SetLevel(stream schema.Stream, percent int) {
	if stream == schema.StreamMain {
		s.SetVolume(volume.Gain(percent))
		return
	}
	s.bus.SetGain(stream, volume.Gain(percent))
}

func (s *Speaker) Mute(stream schema.Stream, on bool) { s.bus.Mute(stream, on) }

func (s *Speaker) Muted(stream schema.Stream) bool { return s.bus.Muted(stream) }

func (s *Speaker) Stats() (queued int, splices, underruns, dropped uint64) {
	return s.Queued(), s.splices.Load(), s.underruns.Load(), s.deaf.Load()
}
