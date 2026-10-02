package boot

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"
	"time"

	"github.com/ygelfand/LANovo/internal/android/slot"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/gpu"
	"github.com/ygelfand/LANovo/internal/hardware/touch"
	"github.com/ygelfand/LANovo/internal/lib/safe"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/reveal"
)

// logoFor is how long the logo is shown before the screen starts saying what it is waiting for.
// Long enough to be a greeting, short enough that a device that is stuck says so quickly.
const logoFor = 3 * time.Second

// listFor is the least time the list stays up once it has appeared. A device that comes up in a
// second would otherwise flash it past unread, and what it says is the only account anyone gets
// of what happened at start-up.
const listFor = 3 * time.Second

const settle = 500 * time.Millisecond

const (
	frame    = time.Second / 60
	traceBy  = 2000 * time.Millisecond
	traceFor = 1400 * time.Millisecond
	stepFor  = 350 * time.Millisecond
	moveFor  = 700 * time.Millisecond
	finish   = 1100 * time.Millisecond
)

// startSplash runs the reveal, then what the device is still waiting for, and lets go once everything is up.
func startSplash(ctx context.Context) {
	safe.Go("splash", func() {
		claim := display.Get().Claim(display.PriorityBoot)
		defer claim.Release()

		w, h := viewed(ctx)
		rv := reveal.New(chosen())
		layer, err := gpu.Open(w, h)
		if err != nil {
			slog.Error("the boot animation could not start", "err", err, "size", fmt.Sprintf("%dx%d", w, h))
		} else {
			defer layer.Close()
			if err := layer.Place(ui.Rect{W: w, H: h}); err != nil {
				slog.Error("placing the boot animation", "err", err)
			}
		}

		claim.Show(DrawLogo)
		slog.Info("splash claimed")

		var skipped atomic.Bool
		var showing atomic.Bool
		skip := Skip(w, h)
		stopSkip := touch.Get().Contacts.Listen(func(c touch.Contact) {
			if c.Phase == touch.Down && showing.Load() && skip.Contains(c.X, c.Y) {
				slog.Info("boot screen skipped")
				skipped.Store(true)
			}
		})
		defer stopSkip()

		began := time.Now()
		tick := time.NewTicker(frame)
		defer tick.Stop()

		var m reveal.Moment
		var listed, done, finishing, stepped time.Time
		shown := 0
		waited := false
		said := "\x00"
		fresh := true

		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
			now := time.Now()
			m.At = now.Sub(began)
			progress := component.Default().Progress()

			if m.At > traceBy {
				if shown < up(progress) && now.Sub(stepped) >= stepFor {
					shown++
					stepped = now
				}
				want := 1.0
				if len(progress) > 0 && !skipped.Load() {
					want = float64(shown) / float64(len(progress))
				}
				m.Trace = math.Min(want, m.Trace+frame.Seconds()/traceFor.Seconds())
			}

			if m.At >= logoFor {
				m.Header = math.Min(1, m.Header+frame.Seconds()/moveFor.Seconds())
			}
			if m.Header >= 1 {
				if listed.IsZero() {
					listed = now
				}
				showing.Store(!Settled(progress))
				if text := summary(progress); text != said {
					said = text
					claim.Show(func(p *display.Panel) error { return DrawBoot(p, progress) })
					slog.Info("coming up", "waiting", text)
				}
			}

			if !listed.IsZero() && !Settled(progress) {
				waited = true
			}
			if !handOver() {
				done = time.Time{}
			} else if done.IsZero() {
				done = now
			}
			if finishing.IsZero() && !listed.IsZero() && !done.IsZero() && now.Sub(done) >= settle && (!waited || now.Sub(listed) >= listFor) && (Settled(progress) || skipped.Load()) {
				finishing = now
			}
			if !finishing.IsZero() && m.Trace >= 1 {
				m.Ready = math.Min(1, m.Ready+frame.Seconds()/finish.Seconds())
			}

			if layer != nil {
				if err := rv.Shade(layer, fresh, w, h, m); err != nil {
					slog.Error("the boot animation stopped", "err", err)
					layer.Close()
					layer = nil
				}
				fresh = false
			}
			if m.Ready >= 1 || (layer == nil && !finishing.IsZero()) {
				break
			}
		}
		claim.Show(func(p *display.Panel) error {
			ui.Fill(ui.Of(p), chosen().Background)
			return nil
		})
		time.Sleep(2 * frame)
		slog.Info("ready")

		// Past the boot screen is the definition of a good boot: everything that holds is up and
		// something else has the panel. Told here rather than from init, so a slot that comes up
		// without lanovod is not recorded as one that worked.
		if err := slot.MarkBooted(); err != nil {
			slog.Warn("could not mark the boot good, this slot is spending its retries", "err", err)
		}
	})
}

func viewed(ctx context.Context) (int, int) {
	for range 40 {
		fw, fh := display.Get().Native()
		if w, h := display.Get().Orientation().Size(fw, fh); w > 0 && h > 0 {
			return w, h
		}
		if !wait(ctx, 50*time.Millisecond) {
			break
		}
	}
	return 0, 0
}

func up(progress []component.Progress) int {
	n := 0
	for _, p := range progress {
		if p.Done || p.Failed {
			n++
		}
	}
	return n
}

func handOver() bool { return component.Default().Ready() }

// summary is what the screen currently says, for deciding whether to draw it again.
func summary(progress []component.Progress) string {
	var s string
	for _, p := range progress {
		if !p.Done {
			s += p.Name + ":" + p.Doing + " "
		}
	}
	return s
}

func wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
