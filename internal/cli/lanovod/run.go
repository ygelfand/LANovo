package lanovod

import (
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ygelfand/libcountertop/pkg/runtime/safe"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/boot"
	"github.com/ygelfand/LANovo/internal/layout"
)

var (
	profile      bool
	tryVideoFile string
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the device agent",
		Long: "Shows the logo, takes the panel and the hardware from Android, and stays\n" +
			"resident. init starts lanovod from on post-fs-data with no arguments, which\n" +
			"means this.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			closeLog := openLog()
			defer closeLog()

			if err := board.Detected(); err != nil {
				slog.Warn("unknown board", "err", err, "using", board.Current().Name)
			} else {
				b := board.Current()
				slog.Info("board", "name", b.Name, "model", b.Model, "soc", b.SoC)
			}

			if profile {
				startPprof()
			}

			if tryVideoFile != "" {
				safe.Go("video trial", func() {
					time.Sleep(15 * time.Second)
					if err := tryVideo(tryVideoFile); err != nil {
						slog.Error("the video trial failed", "err", err)
					}
				})
			}

			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic", "panic", r, "stack", string(debug.Stack()))
					closeLog()
					panic(r)
				}
			}()

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			hangup := make(chan os.Signal, 4)
			signal.Notify(hangup, syscall.SIGHUP)
			go func() {
				for range hangup {
					slog.Warn("SIGHUP (ignored)")
				}
			}()

			// init's ctl.stop marks a service so class_start skips it.
			for _, svc := range layout.Displace {
				if err := stopService(svc); err != nil {
					slog.Warn("could not stop", "service", svc, "err", err)
				}
			}

			return boot.Run(ctx)
		},
	}

	cmd.Flags().BoolVar(&profile, "profile", false, "open the profiler on "+pprofAddr)
	cmd.Flags().StringVar(&tryVideoFile, "try-video", "",
		"play an H.264 (Annex-B with AUDs) or IVF file on the video pipe, 15s after start")
	return cmd
}
