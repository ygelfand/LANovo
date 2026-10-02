package speaker

import (
	"context"
	"encoding/binary"
	"log/slog"
	"math"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/alsa"
)

// Run writes to the card until ctx is canceled, a period at a time.
//
// It keeps writing after the audio stops rather than going quiet with the loop: the card is what
// paces everything, and a loop that only runs when there is something to say has to be restarted
// and resynchronized every time, which is heard as a click at the start of each sound.
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
			// Nothing to write to. Play is already counting what that costs and the supervisor is
			// what reopens the card, so this waits a buffer's worth rather than spinning.
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

// Source is asked for the frames the card is about to play, addressed by absolute output frame
// index. A room playing along with the rest of the house needs that: where audio lands has to follow
// from when the server said to play it, not from when it happened to arrive.
type Source interface {
	// Render fills out with the frames starting at output frame index at, silence where it has none.
	Render(at uint64, out []int16)
}

// Attach sets the Source, or clears it with nil. Only one at a time: two things placing audio by
// absolute frame would be two things deciding what the room plays.
func (s *Speaker) Attach(src Source) {
	s.srcMu.Lock()
	defer s.srcMu.Unlock()
	s.src = src
}

// Written is the output frame index of the next frame to go to the card.
func (s *Speaker) Written() uint64 { return s.written.Load() }

// render asks the Source for this period. Called only from the write loop, so the buffer is reused.
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

// fill puts one period into buf, from the queue where there is any and silence where there is not.
func (s *Speaker) fill(buf []byte) {
	chunk := s.take()
	want := period * Channels

	// A source places its audio by frame index rather than queueing it, so it is mixed in here
	// rather than taken from the queue. Both at once is a chime over a stream, which is right.
	placed := s.render()

	// The queue emptied part way through this buffer, so silence is spliced into whatever was
	// playing. At the end of a sound that is expected; repeatedly during one means audio is arriving
	// slower than it plays out.
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

// open reports whether there is a card to write to.
func (s *Speaker) open() bool { return s.device() != nil }

func (s *Speaker) device() *alsa.Playback {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.out
}

// SetVolume sets the playback gain, nought to one. It is applied as the queue drains rather than as
// it is filled, so turning the dial reaches audio already waiting.
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

// Volume is the playback gain.
func (s *Speaker) Volume() float32 {
	return math.Float32frombits(s.level.Load())
}

// curve is the device's own volume curve, copied from MEDIA_VOLUME_CURVE in
// /system/etc/volume_tables.xml: volume index against attenuation in millibels.
//
// Loudness is roughly logarithmic, so a percentage used as amplitude directly puts everything
// useful in the bottom of the slider — half would be six decibels down, which is barely quieter.
// This is what the panel's own framework played at each index, so it is what the hardware was set
// up to sound like.
//
// One curve for every stream, which the tables agree with: audio_policy_volumes.xml sends music on
// the speaker here, and the system and notification curves for the speaker have the same points.
// They only differ for a headset, which this device has no socket for.
var curve = []struct{ at, mB int }{
	{1, -5800}, {20, -4000}, {60, -1700}, {100, 0},
}

// Gain is what a percentage sounds like, as a multiplier. Between the curve's points it
// interpolates, which is what the framework did with the same table.
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

// Stats is what to look at when a device is playing but sounds wrong. Splices are buffers the queue
// only part filled, underruns ones it did not reach at all while something was playing, and dropped
// is audio offered with no card open.
func (s *Speaker) Stats() (queued int, splices, underruns, dropped uint64) {
	return s.Queued(), s.splices.Load(), s.underruns.Load(), s.deaf.Load()
}
