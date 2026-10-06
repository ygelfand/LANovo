package boot

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/ygelfand/LANovo/internal/android/slot"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/gpu"
	"github.com/ygelfand/LANovo/internal/hardware/touch"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/reveal"
	bootview "github.com/ygelfand/libcountertop/pkg/display/boot"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
	"github.com/ygelfand/libcountertop/pkg/runtime/startup"
)

const (
	frame     = startup.Frame
	viewWait  = 10 * time.Second
	viewEvery = 50 * time.Millisecond
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
		skip := bootview.Skip(w, h)
		stopSkip := touch.Get().Contacts.Listen(func(c touch.Contact) {
			if c.Phase == touch.Down && showing.Load() && skip.Contains(c.X, c.Y) {
				slog.Info("boot screen skipped")
				skipped.Store(true)
			}
		})
		defer stopSkip()

		scene := openBootScene(w, h)
		if scene != nil {
			defer func() {
				if scene != nil {
					scene.Close()
				}
			}()
		}
		fresh := true
		completed := startup.Run(ctx, startup.Hooks{
			Progress: func() []startup.Progress { return component.Default().Progress() },
			Ready:    handOver, Skipped: skipped.Load, OfferSkip: showing.Store,
			Draw: func(progress []startup.Progress) {
				if scene != nil {
					if err := scene.Draw(progress, true); err == nil {
						return
					} else {
						slog.Error("drawing boot progress", "err", err)
						scene.Close()
						scene = nil
					}
				}
				claim.Show(func(p *display.Panel) error { return DrawBoot(p, progress) })
			},
			Changed: func(text string) { slog.Info("coming up", "waiting", text) },
			Animate: func(m startup.Moment) bool {
				if layer == nil {
					return false
				}
				if err := rv.Shade(layer, fresh, w, h, reveal.Moment{At: m.At, Trace: m.Trace, Header: m.Header, Ready: m.Ready}); err != nil {
					slog.Error("the boot animation stopped", "err", err)
					layer.Close()
					layer = nil
					return false
				}
				fresh = false
				return true
			},
		})
		if !completed {
			return
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
	for range int(viewWait / viewEvery) {
		fw, fh := display.Get().Native()
		if w, h := display.Get().Orientation().Size(fw, fh); w > 0 && h > 0 {
			return w, h
		}
		if !wait(ctx, viewEvery) {
			break
		}
	}
	return 0, 0
}

func handOver() bool { return component.Default().Ready() }

// summary is what the screen currently says, for deciding whether to draw it again.
func summary(progress []component.Progress) string { return startup.Summary(progress) }

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
