package speaker

import (
	"log/slog"
	"math"

	"github.com/ygelfand/libcountertop/pkg/audio/analysis"
)

// Play queues interleaved stereo samples.
//
// Audio offered while the card is not open is dropped rather than queued. Nothing is draining the
// queue then, so it would grow for as long as the speaker stayed away, and a device that cannot
// play should say so in the log rather than in memory.
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

// Take empties the queue and hands back what had not been played, so a sound that yields to another
// can carry on from where it was rather than skipping whatever it had queued.
func (s *Speaker) Take() []int16 {
	s.qmu.Lock()
	defer s.qmu.Unlock()

	pending := s.pending
	s.pending = nil
	return pending
}

// Adjust rewrites the queue in place. Taking it out and putting it back would leave it empty in
// between, and a buffer filled then pads with silence.
func (s *Speaker) Adjust(rewrite func([]int16)) {
	s.qmu.Lock()
	defer s.qmu.Unlock()

	rewrite(s.pending)
}

// PlayVoice queues 16 kHz mono, which is what a voice pipeline sends. The codec only takes 48 kHz
// stereo, so it is stretched and duplicated across both channels. The resampler carries state
// between calls, so a reply delivered in chunks is one continuous signal.
func (s *Speaker) PlayVoice(mono []int16) {
	s.voiceMu.Lock()
	if s.voice == nil {
		s.voice, s.using = NewResampler(ResampleSinc)
	}
	out := s.voice.Run(mono, make([]int16, 0, len(mono)*VoiceUpsample*Channels))
	s.voiceMu.Unlock()

	s.Play(out)
}

// SetResampling picks how voice is stretched and reports what it settled on, which is the filter
// for anything this build does not have. It takes effect on the next reply.
func (s *Speaker) SetResampling(r Resampling) Resampling {
	s.voiceMu.Lock()
	defer s.voiceMu.Unlock()

	s.voice, s.using = NewResampler(r)
	return s.using
}

// Resampling is the one in use.
func (s *Speaker) Resampling() Resampling {
	s.voiceMu.Lock()
	defer s.voiceMu.Unlock()

	if s.voice == nil {
		return ResampleSinc
	}
	return s.using
}

// Clipped counts voice samples the resampler pushed past full scale. Interpolation overshoots a
// transient, and because volume is applied further along that distortion survives being turned down.
func (s *Speaker) Clipped() uint64 {
	s.voiceMu.Lock()
	defer s.voiceMu.Unlock()

	if s.voice == nil {
		return 0
	}
	return s.voice.Clipped()
}

// Drain discards anything queued but not yet played, for a barge-in. The resampler's history goes
// with it: whatever comes next is a different utterance.
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

// Queued is how many frames are waiting.
func (s *Speaker) Queued() int {
	s.qmu.Lock()
	defer s.qmu.Unlock()

	return len(s.pending) / Channels
}

// Overlay mixes samples into what is already queued, extending the queue if they outlast it. Sums
// are clamped: two things at once are louder than either, and wrapping would turn that into a crack.
func (s *Speaker) Overlay(samples []int16) {
	if !s.open() {
		s.Play(samples)
		return
	}

	s.qmu.Lock()
	defer s.qmu.Unlock()

	s.pending = mix(s.pending, samples)
}

// take pulls up to one period off the front of the queue.
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

// mix sums add into the front of into, extending it if add outlasts it.
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

// Scale multiplies samples in place, clamping rather than wrapping.
var Scale = analysis.Scale
