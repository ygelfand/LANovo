package speaker

import (
	"log/slog"

	"github.com/ygelfand/libcountertop/pkg/audio/upsample"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

const VoiceRate = upsample.VoiceRate

func (s *Speaker) Play(samples []int16) { s.PlayStream(schema.StreamMedia, samples) }

func (s *Speaker) PlayStream(stream schema.Stream, samples []int16) {
	if !s.open() {
		if n := s.deaf.Add(1); n == 1 || n%100 == 0 {
			slog.Warn("audio dropped, no playback device", "times", n)
		}
		return
	}
	s.bus.Push(stream, samples)
}

func (s *Speaker) Take() []int16 { return s.bus.Take(schema.StreamMedia) }

func (s *Speaker) Adjust(rewrite func([]int16)) { s.bus.Adjust(schema.StreamMedia, rewrite) }

// The codec only takes 48 kHz stereo.
func (s *Speaker) PlayVoice(mono []int16) {
	s.voiceMu.Lock()
	if s.voice == nil {
		s.voice, s.using = upsample.New(upsample.Sinc, Rate)
	}
	out := s.voice.Run(mono, make([]int16, 0, len(mono)*Rate/VoiceRate*Channels))
	s.voiceMu.Unlock()

	s.PlayStream(schema.StreamVoice, out)
}

func (s *Speaker) SetResampling(k upsample.Kind) upsample.Kind {
	s.voiceMu.Lock()
	defer s.voiceMu.Unlock()

	s.voice, s.using = upsample.New(k, Rate)
	return s.using
}

func (s *Speaker) Resampling() upsample.Kind {
	s.voiceMu.Lock()
	defer s.voiceMu.Unlock()

	if s.voice == nil {
		return upsample.Sinc
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
	s.bus.Clear()

	s.voiceMu.Lock()
	if s.voice != nil {
		s.voice.Reset()
	}
	s.voiceMu.Unlock()
}

func (s *Speaker) Queued() int { return s.bus.Len() / Channels }

func (s *Speaker) Overlay(stream schema.Stream, samples []int16) {
	if !s.open() {
		s.PlayStream(stream, samples)
		return
	}
	s.bus.Overlay(stream, samples)
}
