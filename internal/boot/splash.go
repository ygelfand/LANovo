package boot

import (
	"context"
	"log/slog"

	bootview "github.com/ygelfand/libcountertop/pkg/display/boot"
	"github.com/ygelfand/libcountertop/pkg/display/panel"
	"github.com/ygelfand/libcountertop/pkg/display/theme"
	"github.com/ygelfand/libcountertop/pkg/display/ui"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
	"github.com/ygelfand/libcountertop/pkg/runtime/startup"

	"github.com/ygelfand/LANovo/internal/android/slot"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/gpu"
	hwtouch "github.com/ygelfand/LANovo/internal/hardware/touch"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/ui/reveal"
)

func startSplash(ctx context.Context) {
	safe.Go("splash", func() {
		claim := display.Get().Claim(panel.PriorityBoot)
		defer claim.Release()

		w, h := bootview.Viewed(ctx, display.Get())
		claim.Show(DrawLogo)
		slog.Info("splash claimed")

		completed := bootview.Splash{
			Claim:    claim,
			Display:  display.Get(),
			Layers:   gpu.Get(),
			Contacts: &hwtouch.Get().Contacts,
			Reveal:   reveal.New(chosen()),
			Theme:    func() theme.Theme { return chosen() },
			Version:  layout.Version,
			Progress: func() []startup.Progress { return component.Default().Progress() },
			Ready:    handOver,
			Raster:   DrawBoot,
			Blank: func(p *panel.Panel) error {
				ui.Fill(ui.Of(p), chosen().Background)
				return nil
			},
		}.Play(ctx, w, h)
		if !completed {
			return
		}

		if err := slot.MarkBooted(); err != nil {
			slog.Warn("could not mark the boot good, this slot is spending its retries", "err", err)
		}
	})
}

func handOver() bool { return component.Default().Ready() }
