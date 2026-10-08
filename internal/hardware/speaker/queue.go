package speaker

import (
	"log/slog"
	"math"

	"github.com/ygelfand/libcountertop/pkg/audio/analysis"
)

func (s *Speaker) Play(samples []int16) {
	if !s.open() {
		if n := s.deaf.Add(1); n == 1 || n%100 == 0 {
			slog.Warn("audio dropped, no playback device", "times", n)
		}
		return
	}

	s.qmu.Lock()
	s.pending = append(s.pending, samples...)
	s.qmu.Unlock()
}

func (s *Speaker) Take() []int16 {
	s.qmu.Lock()
	defer s.qmu.Unlock()

	pending := s.pending
	s.pending = nil
	return pending
}

func (s *Speaker) Adjust(rewrite func([]int16)) {
	s.qmu.Lock()
	defer s.qmu.Unlock()

	rewrite(s.pending)
}

// The codec only takes 48 kHz stereo.
func (s *Speaker) PlayVoice(mono []int16) {
	s.voiceMu.Lock()
	if s.voice == nil {
		s.voice, s.using = NewResampler(ResampleSinc)
	}
	out := s.voice.Run(mono, make([]int16, 0, len(mono)*VoiceUpsample*Channels))
	s.voiceMu.Unlock()

	s.Play(out)
}

func (s *Speaker) SetResampling(r Resampling) Resampling {
	s.voiceMu.Lock()
	defer s.voiceMu.Unlock()

	s.voice, s.using = NewResampler(r)
	return s.using
}

func (s *Speaker) Resampling() Resampling {
	s.voiceMu.Lock()
	defer s.voiceMu.Unlock()

	if s.voice == nil {
		return ResampleSinc
	}
	return s.using
}

func (s *Speaker) Clipped() uint64 {
	s.voiceMu.Lock()
	defer s.voiceMu.Unlock()

	if s.voice == nil {
		return 0
	}
	return s.voice.Clipped()
}

func (s *Speaker) Drain() {
	s.qmu.Lock()
	s.pending = nil
	s.qmu.Unlock()

	s.voiceMu.Lock()
	if s.voice != nil {
		s.voice.Reset()
	}
	s.voiceMu.Unlock()
}

func (s *Speaker) Queued() int {
	s.qmu.Lock()
	defer s.qmu.Unlock()

	return len(s.pending) / Channels
}

func (s *Speaker) Overlay(samples []int16) {
	if !s.open() {
		s.Play(samples)
		return
	}

	s.qmu.Lock()
	defer s.qmu.Unlock()

	s.pending = mix(s.pending, samples)
}

func (s *Speaker) take() []int16 {
	s.qmu.Lock()
	defer s.qmu.Unlock()

	n := min(len(s.pending), period*Channels)
	chunk := s.pending[:n]

	s.pending = s.pending[n:]
	if len(s.pending) == 0 {
		s.pending = nil
	}
	return chunk
}

func mix(into, add []int16) []int16 {
	for i, v := range add {
		if i >= len(into) {
			return append(into, add[i:]...)
		}
		into[i] = clamp(int32(into[i]) + int32(v))
	}
	return into
}

func clamp(v int32) int16 {
	switch {
	case v > math.MaxInt16:
		return math.MaxInt16
	case v < math.MinInt16:
		return math.MinInt16
	}
	return int16(v)
}

var Scale = analysis.Scale
