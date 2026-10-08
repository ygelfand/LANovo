package speaker

import (
	"context"
	"encoding/binary"
	"log/slog"
	"math"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/alsa"
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

		if _, err := out.Write(buf); err != nil {
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

type Source interface {
	Render(at uint64, out []int16)
}

func (s *Speaker) Attach(src Source) {
	s.srcMu.Lock()
	defer s.srcMu.Unlock()
	s.src = src
}

func (s *Speaker) Written() uint64 { return s.written.Load() }

func (s *Speaker) render() []int16 {
	s.srcMu.Lock()
	src := s.src
	s.srcMu.Unlock()

	if src == nil {
		return nil
	}

	want := period * Channels
	if cap(s.srcBuf) < want {
		s.srcBuf = make([]int16, want)
	}
	s.srcBuf = s.srcBuf[:want]
	clear(s.srcBuf)

	src.Render(s.written.Load(), s.srcBuf)
	return s.srcBuf
}

func (s *Speaker) fill(buf []byte) {
	chunk := s.take()
	want := period * Channels

	placed := s.render()

	switch {
	case len(chunk) > 0 && len(chunk) < want:
		s.splices.Add(1)
	case len(chunk) == 0 && s.fed:
		s.underruns.Add(1)
	}
	s.fed = len(chunk) > 0 || len(placed) > 0

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
			var sum int32
			if i < len(chunk) {
				sum += int32(chunk[i])
			}
			if i < len(placed) {
				sum += int32(placed[i])
			}
			if tap != nil {
				s.tapBuf[i] = clamp(int32(float32(sum) * gain))
			}
			sums[c] = sum
			both += sum
		}

		s.mono[f] = clamp(int32(float32(both) / Channels * gain))
		for c := range Channels {
			v := clamp(int32(float32(both) / Channels * out))
			if stereo {
				v = clamp(int32(float32(sums[c]) * out))
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

// MEDIA_VOLUME_CURVE from /system/etc/volume_tables.xml: index against attenuation in millibels.
var curve = []struct{ at, mB int }{
	{1, -5800}, {20, -4000}, {60, -1700}, {100, 0},
}

func Gain(percent int) float32 {
	percent = min(max(percent, 0), 100)
	if percent == 0 {
		return 0
	}

	mB := float64(curve[0].mB)
	for i := 1; i < len(curve); i++ {
		lo, hi := curve[i-1], curve[i]
		if percent > hi.at {
			continue
		}
		if percent <= lo.at {
			break
		}

		across := float64(percent-lo.at) / float64(hi.at-lo.at)
		mB = float64(lo.mB) + across*float64(hi.mB-lo.mB)
		break
	}
	return float32(math.Pow(10, mB/2000))
}

func (s *Speaker) Stats() (queued int, splices, underruns, dropped uint64) {
	return s.Queued(), s.splices.Load(), s.underruns.Load(), s.deaf.Load()
}
