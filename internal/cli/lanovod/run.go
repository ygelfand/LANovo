package lanovod

import (
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/boot"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/lib/safe"
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

			if b, err := board.Detect(prop.Local); err != nil {
				slog.Warn("unknown board", "err", err, "using", board.Current().Name)
			} else {
				board.Set(b)
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

			// init discards our stderr, so a panic would otherwise vanish and look like a silent
			// restart. Log it, then let it kill the process as it would have.
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic", "panic", r, "stack", string(debug.Stack()))
					closeLog()
					panic(r)
				}
			}()

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			// Something sends SIGHUP, which by default kills the process. Logged rather than
			// ignored outright, because who sends it is not yet known.
			hangup := make(chan os.Signal, 4)
			signal.Notify(hangup, syscall.SIGHUP)
			go func() {
				for range hangup {
					slog.Warn("SIGHUP (ignored)")
				}
			}()

			// The panel cannot be opened while SurfaceFlinger has it. From post-fs-data these have
			// not started yet, and init's stop marks them so class_start skips them.
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
