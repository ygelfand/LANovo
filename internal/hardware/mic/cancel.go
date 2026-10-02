package mic

import (
	"log/slog"
	"math"
	"slices"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/lib/aec"
)

const (
	cancelTaps = 1024
	speexFrame = 64

	echoSuppressActive = -40

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
			slog.Info("echo reference moved", "from", a.offset, "to", median)
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

type canceller struct {
	left, right *aec.MDF
	ref         Downsampler
	refBuf      []int16

	active     bool
	best, last [2]float64
	refE, micE float64
	blocks     int
	since      time.Time
}

func newCanceller() *canceller {
	l, err := aec.NewMDF(speexFrame, cancelTaps, Voice)
	if err != nil {
		slog.Error("echo cancellation unavailable", "err", err)
		return nil
	}
	r, err := aec.NewMDF(speexFrame, cancelTaps, Voice)
	if err != nil {
		slog.Error("echo cancellation unavailable", "err", err)
		return nil
	}
	return &canceller{left: l, right: r}
}

func (c *canceller) begin(ref, left, right []int16, adapting bool) {
	if !c.active {
		c.active, c.since = true, time.Now()
		slog.Info("echo cancellation running", "engine", "speex", "taps", cancelTaps)
	}
	c.left.SetAdapting(adapting)
	c.right.SetAdapting(adapting)
	c.refE += power(ref)
	c.micE += (power(left) + power(right)) / 2
	c.blocks++
}

func (c *canceller) end() {
	for i, f := range []*aec.MDF{c.left, c.right} {
		c.last[i] = f.ERLE()
		c.best[i] = max(c.best[i], c.last[i])
	}
}

func (c *canceller) idle() {
	if !c.active {
		return
	}
	slog.Info("echo cancellation idle",
		"best_db", []float64{round1(c.best[0]), round1(c.best[1])},
		"last_db", []float64{round1(c.last[0]), round1(c.last[1])},
		"ref_dbfs", meanDBFS(c.refE, c.blocks), "mic_dbfs", meanDBFS(c.micE, c.blocks),
		"seconds", round1(time.Since(c.since).Seconds()))
	c.active = false
	c.best, c.last = [2]float64{}, [2]float64{}
	c.refE, c.micE, c.blocks = 0, 0, 0
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func meanDBFS(energy float64, blocks int) float64 {
	if blocks == 0 || energy <= 0 {
		return -120
	}
	return round1(10 * math.Log10(energy/float64(blocks)/(32768*32768)))
}

func power(s []int16) float64 {
	if len(s) == 0 {
		return 0
	}
	var sum float64
	for _, v := range s {
		sum += float64(v) * float64(v)
	}
	return sum / float64(len(s))
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
		c.idle()
		return nil
	}
	from, ok := a.start(first)
	if !ok {
		return nil
	}
	if cap(c.refBuf) < frames {
		c.refBuf = make([]int16, frames)
	}
	c.refBuf = c.refBuf[:frames]
	if !speaker.Get().Echo(from, c.refBuf) {
		return nil
	}
	return c.ref.Of(c.refBuf)
}

func newPreprocessors(c *canceller) [2]*aec.Preprocessor {
	var out [2]*aec.Preprocessor
	for i := range out {
		var echo *aec.MDF
		if c != nil {
			echo = []*aec.MDF{c.left, c.right}[i]
		}
		p, err := aec.NewPreprocessor(speexFrame, Voice, echo)
		if err != nil {
			slog.Error("noise suppression unavailable", "err", err)
			return [2]*aec.Preprocessor{}
		}
		p.EchoSuppressActive = echoSuppressActive
		out[i] = p
	}
	return out
}

func (m *Mics) clean(left, right, ref []int16) ([]int16, []int16, bool) {
	c := m.cancel
	n := len(left)
	cancel := c != nil && ref != nil && len(ref) == n && len(right) == n && n%speexFrame == 0
	denoising := m.denoising.Load() && m.pre[0] != nil && n%speexFrame == 0 && len(right) == n
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
	if cancel {
		c.begin(ref, left, right, m.adapting.Load())
	}
	in := [2][]int16{left, right}
	out := [2][]int16{slices.Clone(left), slices.Clone(right)}
	mdf := [2]*aec.MDF{}
	if c != nil {
		mdf = [2]*aec.MDF{c.left, c.right}
	}
	for k := 0; k+speexFrame <= n; k += speexFrame {
		for side := range 2 {
			chunk := out[side][k : k+speexFrame]
			if cancel {
				got, err := mdf[side].Process(in[side][k:k+speexFrame], ref[k:k+speexFrame])
				if err != nil {
					slog.Error("echo cancellation failed", "err", err)
					return left, right, false
				}
				copy(chunk, got)
			}
			if denoising {
				if cancel {
					m.pre[side].SetEcho(mdf[side])
				} else {
					m.pre[side].SetEcho(nil)
				}
				m.pre[side].Run(chunk)
			}
		}
	}
	if cancel {
		c.end()
	}
	return out[0], out[1], cancel
}

func (m *Mics) SetDenoising(on bool) { m.denoising.Store(on) }
