package lanovod

import (
	"fmt"
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
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/layout"
)

var tryVideoFile string

func newRunCmd() *cobra.Command {
	var keepAndroid bool
	var androidPCM string
	var surfaceSocket string
	var skipParts bool
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
			if surfaceSocket != "" {
				if os.Getuid() != 0 {
					return fmt.Errorf("SurfaceFlinger display requires root")
				}
				display.SetHelperSocket(surfaceSocket)
			}
			if skipParts && (keepAndroid || surfaceSocket == "") {
				return fmt.Errorf("--skip-parts requires a takeover run with --surface-socket")
			}

			if androidPCM != "" && keepAndroid && os.Getuid() == 2000 {
				// This path runs only the explicitly selected X501F audio trial.
				// The shell domain cannot read the framebuffer used by auto-detection.
				board.Set(board.X501F)
				slog.Info("using SD-X501F profile for shell audio trial")
			} else if err := board.Detected(); err != nil {
				slog.Warn("unknown board", "err", err, "using", board.Current().Name)
			} else {
				b := board.Current()
				slog.Info("board", "name", b.Name, "model", b.Model, "soc", b.SoC)
			}

			if androidPCM != "" {
				if board.Current().Name != "sd-x501f" {
					return fmt.Errorf("--android-pcm requires SD-X501F")
				}
				speaker.Get().AndroidPCM(androidPCM)
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
				if keepAndroid {
					break
				}
				// The SD-X501F speaker uses the OpenQ624 WSA881x route. Keep
				// Android's audio HAL alive so it can initialize that route.
				if board.Current().Name == "sd-x501f" &&
					(svc == "audioserver" || svc == "audio-hal-2-0") {
					continue
				}
				if err := stopService(svc); err != nil {
					slog.Warn("could not stop", "service", svc, "err", err)
				}
			}
			if !keepAndroid && board.Current().Name == board.X501F.Name {
				if err := stopService("bootanim"); err != nil {
					slog.Warn("could not stop", "service", "bootanim", "err", err)
				}
			}

			return boot.Run(ctx, keepAndroid, androidPCM != "" && keepAndroid && surfaceSocket == "", skipParts)
		},
	}

	cmd.Flags().StringVar(&tryVideoFile, "try-video", "",
		"play an H.264 (Annex-B with AUDs) or IVF file on the video pipe, 15s after start")
	cmd.Flags().BoolVar(&keepAndroid, "keep-android", false,
		"leave Android services running during a manual diagnostic session")
	cmd.Flags().StringVar(&androidPCM, "android-pcm", "",
		"stream speaker PCM to this Android AudioTrack helper")
	cmd.Flags().StringVar(&surfaceSocket, "surface-socket", "",
		"draw through the specified SurfaceFlinger helper socket")
	cmd.Flags().BoolVar(&skipParts, "skip-parts", false,
		"skip persistent system-part installation during an ephemeral takeover trial")
	return cmd
}
