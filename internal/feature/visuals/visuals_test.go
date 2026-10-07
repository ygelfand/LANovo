package visuals

import (
	esphome "github.com/ygelfand/go-esphome-device"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

type rig struct {
	frames  chan []int16
	stopped atomic.Int32
	muted   atomic.Bool

	mu  sync.Mutex
	tap speaker.Tap
}

func newRig() (*rig, *Visuals) {
	r := &rig{frames: make(chan []int16, 64)}
	v := newVisuals(
		func() (<-chan []int16, func()) { return r.frames, func() { r.stopped.Add(1) } },
		r.muted.Load,
		func(t speaker.Tap) { r.mu.Lock(); r.tap = t; r.mu.Unlock() },
	)
	return r, v
}

func (r *rig) tapped() speaker.Tap {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tap
}

func tone(n int, amp float64) []int16 {
	out := make([]int16, n)
	for i := range out {
		out[i] = int16(amp * math.MaxInt16 * math.Sin(2*math.Pi*300*float64(i)/16000))
	}
	return out
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	until := time.Now().Add(2 * time.Second)
	for !ok() {
		if time.Now().After(until) {
			t.Fatalf("never: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestTheTapsRunOnlyWhileHeld(t *testing.T) {
	r, v := newRig()
	if r.tapped() != nil {
		t.Fatal("tapped before anything held")
	}

	a := v.Hold()
	b := v.Hold()
	if r.tapped() == nil {
		t.Fatal("not tapped while held")
	}
	a()
	a()
	if r.tapped() == nil {
		t.Fatal("one release of two stopped the tap")
	}
	b()
	if r.tapped() != nil {
		t.Error("still tapped after the last release")
	}
	eventually(t, "the mic listener stopped", func() bool { return r.stopped.Load() == 1 })
}

func TestTheMicIsHeard(t *testing.T) {
	r, v := newRig()
	defer v.Hold()()

	for range 20 {
		r.frames <- tone(320, 0.5)
	}
	eventually(t, "the mic level rose", func() bool { return v.Input().Mic.Level > 0.1 })
}

func TestAMutedMicIsNotHeard(t *testing.T) {
	r, v := newRig()
	r.muted.Store(true)
	defer v.Hold()()

	for range 20 {
		r.frames <- tone(320, 0.5)
	}
	time.Sleep(100 * time.Millisecond)
	if got := v.Input().Mic.Level; got != 0 {
		t.Errorf("a muted mic read level %v", got)
	}
}

func TestMutingDropsTheLevelInsteadOfHoldingIt(t *testing.T) {
	r, v := newRig()
	defer v.Hold()()

	for range 20 {
		r.frames <- tone(320, 0.5)
	}
	eventually(t, "the mic level rose", func() bool { return v.Input().Mic.Level > 0.1 })

	r.muted.Store(true)
	for range 200 {
		r.frames <- tone(320, 0.5)
	}
	eventually(t, "the level fell once muted", func() bool { return v.Input().Mic.Level < 0.01 })
}

func TestTheSpeakerIsHeardAndCountsAsReplying(t *testing.T) {
	r, v := newRig()
	defer v.Hold()()

	stereo := make([]int16, 0, 9600)
	for _, s := range tone(4800, 0.5) {
		stereo = append(stereo, s, s)
	}
	for range 5 {
		r.tapped().Offer(stereo)
		time.Sleep(25 * time.Millisecond)
	}
	eventually(t, "the speaker level rose", func() bool {
		x := v.Input()
		return x.Speaker.Level > 0.1 && x.Replying
	})
}

func TestRestoreFallsBackFromAnUnbuiltVisual(t *testing.T) {
	_, v := newRig()
	for _, saved := range []string{"no-such-visual", "pac-man"} {
		cfg := config.Defaults()
		cfg.Visual.Kind = saved
		v.Restore(cfg)
		if v.Kind() != visual.Kind(config.DefaultVisual) {
			t.Errorf("restoring %q chose %q, want %q", saved, v.Kind(), config.DefaultVisual)
		}
		if v.Entities()[0].(*esphome.Select).Get() != v.Kind().Label() {
			t.Errorf(
				"restoring %q left the select on %q",
				saved,
				v.Entities()[0].(*esphome.Select).Get(),
			)
		}
	}

	cfg := config.Defaults()
	cfg.Visual.Kind = string(visual.LCARS)
	v.Restore(cfg)
	if v.Kind() != visual.LCARS || v.Entities()[0].(*esphome.Select).Get() != visual.LCARS.Label() {
		t.Errorf("restoring lcars gave %q / %q", v.Kind(), v.Entities()[0].(*esphome.Select).Get())
	}
}

func TestTheSelectOffersEveryBuiltVisual(t *testing.T) {
	_, v := newRig()
	if len(v.Entities()[0].(*esphome.Select).Options) != len(visual.Built()) {
		t.Fatalf(
			"%d options for %d visuals",
			len(v.Entities()[0].(*esphome.Select).Options),
			len(visual.Built()),
		)
	}
	seen := map[string]bool{}
	for _, o := range v.Entities()[0].(*esphome.Select).Options {
		if seen[o] {
			t.Errorf("%q offered twice", o)
		}
		seen[o] = true
	}
}
