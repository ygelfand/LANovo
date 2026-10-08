package speaker

import (
	"log/slog"

	"github.com/ygelfand/libcountertop/pkg/audio/upsample"
)

const VoiceRate = upsample.VoiceRate

func (s *Speaker) Play(samples []int16) {
	if !s.open() {
		if n := s.deaf.Add(1); n == 1 || n%100 == 0 {
			slog.Warn("audio dropped, no playback device", "times", n)
		}
		return
	}
	s.queue.Push(samples)
}

func (s *Speaker) Take() []int16 { return s.queue.Take() }

func (s *Speaker) Adjust(rewrite func([]int16)) { s.queue.Adjust(rewrite) }

// The codec only takes 48 kHz stereo.
func (s *Speaker) PlayVoice(mono []int16) {
	s.voiceMu.Lock()
	if s.voice == nil {
		s.voice, s.using = upsample.New(upsample.Sinc, Rate)
	}
	out := s.voice.Run(mono, make([]int16, 0, len(mono)*Rate/VoiceRate*Channels))
	s.voiceMu.Unlock()

	s.Play(out)
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
	s.queue.Clear()

	s.voiceMu.Lock()
	if s.voice != nil {
		s.voice.Reset()
	}
	s.voiceMu.Unlock()
}

func (s *Speaker) Queued() int { return s.queue.Len() / Channels }

func (s *Speaker) Overlay(samples []int16) {
	if !s.open() {
		s.Play(samples)
		return
	}
	s.queue.Overlay(samples)
}

func (s *Speaker) take() []int16 { return s.queue.Next(period * Channels) }
