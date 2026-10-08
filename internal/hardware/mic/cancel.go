package mic

import (
	"log/slog"
	"math"
	"slices"
	"time"

	"github.com/ygelfand/libcountertop/pkg/audio/aec"

	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

const (
	lead       = Rate / 500
	estimates  = 64
	relock     = Rate / 1000
	staleAfter = 100 * time.Millisecond
)

type align struct {
	pipeline int64

	seen   [estimates]int64
	n      int
	offset int64
	locked bool
}

func (a *align) observe(capturing uint64, at time.Time, playing uint64, played time.Time) {
	if at.Sub(played) > staleAfter || played.Sub(at) > staleAfter {
		a.n, a.locked = 0, false
		return
	}
	d := int64(playing) - int64(capturing) + int64(math.Round(at.Sub(played).Seconds()*Rate))
	a.seen[a.n%estimates] = d
	a.n++

	sorted := slices.Clone(a.seen[:min(a.n, estimates)])
	slices.Sort(sorted)
	median := sorted[len(sorted)/2]
	if !a.locked || median-a.offset > relock || a.offset-median > relock {
		if a.locked {
			slog.Debug("echo reference moved", "from", a.offset, "to", median)
		}
		a.offset, a.locked = median, true
	}
}

func (a *align) start(first uint64) (uint64, bool) {
	at := int64(first) + a.offset - a.pipeline - lead
	if !a.locked || at < 0 {
		return 0, false
	}
	return uint64(at), true
}

func newCanceller() *aec.Canceller {
	c, err := aec.NewCanceller(Channels, aec.Taps, Voice)
	if err != nil {
		slog.Error("echo cancellation unavailable", "err", err)
		return nil
	}
	return c
}

func interleave(left, right []int16) []int16 {
	out := make([]int16, 2*min(len(left), len(right)))
	for i := range len(out) / 2 {
		out[2*i], out[2*i+1] = left[i], right[i]
	}
	return out
}

func mono(left, right []int16) []int16 {
	out := make([]int16, min(len(left), len(right)))
	for i := range out {
		out[i] = int16((int32(left[i]) + int32(right[i])) / 2)
	}
	return out
}

func (m *Mics) SetAdapting(on bool) { m.adapting.Store(on) }

type EchoFrame struct {
	Mic        [2][]int16
	Reference  []int16
	Out        [2][]int16
	Cancelling bool
	Offset     int64
	Aligned    bool
}

func (m *Mics) reference(a *align, first uint64, frames int, sounding bool) []int16 {
	c := m.cancel
	if c == nil {
		return nil
	}
	if !sounding {
		c.Idle()
		return nil
	}
	from, ok := a.start(first)
	if !ok {
		return nil
	}
	if cap(m.refBuf) < frames {
		m.refBuf = make([]int16, frames)
	}
	m.refBuf = m.refBuf[:frames]
	if !speaker.Get().Echo(from, m.refBuf) {
		return nil
	}
	return m.refDown.Of(m.refBuf)
}

func newPreprocessors(c *aec.Canceller) [2]*aec.Preprocessor {
	var out [2]*aec.Preprocessor
	for i := range out {
		var echo *aec.MDF
		if c != nil {
			echo = c.Filter(i)
		}
		p, err := aec.NewSuppressor(Voice, echo)
		if err != nil {
			slog.Error("noise suppression unavailable", "err", err)
			return [2]*aec.Preprocessor{}
		}
		out[i] = p
	}
	return out
}

func (m *Mics) clean(left, right, ref []int16) ([]int16, []int16, bool) {
	c := m.cancel
	n := len(left)
	cancel := c != nil && ref != nil && len(ref) == n && len(right) == n && n%aec.Block == 0
	denoising := m.denoising.Load() && m.pre[0] != nil && n%aec.Block == 0 && len(right) == n
	switch {
	case denoising && !m.wasDenoising:
		m.pre = newPreprocessors(c)
		m.wasDenoising = true
	case !denoising:
		m.wasDenoising = false
	}
	if !cancel && !denoising {
		return left, right, false
	}
	out := [][]int16{slices.Clone(left), slices.Clone(right)}
	if !cancel {
		for side, p := range m.pre {
			p.SetEcho(nil)
			if err := p.Run(out[side]); err != nil {
				slog.Error("noise suppression failed", "err", err)
			}
		}
		return out[0], out[1], false
	}

	c.SetAdapting(m.adapting.Load())
	var after func(int, []int16)
	if denoising {
		for side, p := range m.pre {
			p.SetEcho(c.Filter(side))
		}
		after = func(side int, block []int16) {
			if err := m.pre[side].Run(block); err != nil {
				slog.Error("noise suppression failed", "err", err)
			}
		}
	}
	if err := c.Process(ref, [][]int16{left, right}, out, after); err != nil {
		slog.Error("echo cancellation failed", "err", err)
		return left, right, false
	}
	return out[0], out[1], true
}

func (m *Mics) SetDenoising(on bool) { m.denoising.Store(on) }
