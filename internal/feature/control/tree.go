package control

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/callcmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/screencmd"

	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/message"
	panel "github.com/ygelfand/LANovo/internal/feature/settings"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/timer"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/logging"
)

func (c *Control) tree() *cobra.Command {
	t := harness.NewTree()
	t.Add(harness.DeviceTask, harness.DeviceCommands(deviceInfo)...)
	t.Add(harness.DeviceTask, telling()...)
	t.Add(harness.InputTask, c.touching()...)
	t.Add(harness.DisplayTask, showing()...)
	t.Add(harness.AudioTask, sounding()...)
	t.Add(harness.MediaTask, casting()...)
	t.Top(settingsCommand(), camera())
	t.Top(radios()...)
	t.Top(homeAssistant()...)
	t.Top(calling()...)
	return t.Root()
}

func Local(args []string) (string, error) {
	return harness.Execute(context.Background(), func() *cobra.Command {
		t := harness.NewTree()
		t.Add(harness.MediaTask, casting()...)
		return t.Root()
	}, args)
}

var says = harness.Says
var group = harness.Group
var does = harness.Does

func (c *Control) touching() []*cobra.Command {
	out := c.inputEngine().Commands()
	return append(
		out,
		does(
			&cobra.Command{
				Use:   "turn mounted|portrait|landscape|left|right|0|90|180|270",
				Short: "Rotate what is drawn",
				Args:  cobra.ExactArgs(1),
			},
			turn,
		),
	)
}

func showing() []*cobra.Command {
	return []*cobra.Command{
		says(&cobra.Command{
			Use:   "shot [PATH]",
			Short: "Take a screenshot",
			Args:  cobra.MaximumNArgs(1),
		}, shot),
		{
			Use:   "size",
			Short: "Say how big the picture is",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				w, h := picture()
				fmt.Fprintf(cmd.OutOrStdout(), "%d %d\n", w, h)
				return nil
			},
		},
		{
			Use:   "stats",
			Short: "How much has been drawn since the start",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				frames, whole, painted := display.Get().Stats()
				fmt.Fprintf(cmd.OutOrStdout(), "frames %d whole %d painted %d megapixels %.1f\n",
					frames, whole, painted, float64(painted)/1e6)
				return nil
			},
		},
		says(&cobra.Command{
			Use:   "gltime FILE [PASSES AMOUNT RADIUS GLOWPASSES SECONDS]",
			Short: "Run a shader file on a full-screen GPU layer for timing",
			Args:  cobra.RangeArgs(1, 6),
		}, gltime),
		says(&cobra.Command{
			Use:   "render KIND FILE [W H [FRAMES]]",
			Short: "Render one visual offscreen after loud frames (50 by default) into a PNG",
			Args:  cobra.RangeArgs(2, 5),
		}, render),
		says(&cobra.Command{
			Use:   "clip KIND DIR [W H [SECONDS [FPS]]]",
			Short: "Render a few seconds of one visual offscreen, to a beat, as numbered PNG frames",
			Args:  cobra.RangeArgs(2, 6),
		}, clip),
		says(&cobra.Command{
			Use:   "thumbs [DIR]",
			Short: "Render every visual offscreen into picker thumbnails, landscape and portrait",
			Args:  cobra.MaximumNArgs(1),
		}, thumbs),
		says(&cobra.Command{
			Use:   "boot FILE SECONDS [TRACE HEADER READY] [THEME] [WxH]",
			Short: "Render one frame of the boot reveal offscreen into a PNG",
			Args:  cobra.RangeArgs(2, 7),
		}, bootFrame),
		says(&cobra.Command{
			Use:   "fps [FRAMES]",
			Short: "Measure the frame rate over a run of frames",
			Args:  cobra.MaximumNArgs(1),
		}, fps),
		does(&cobra.Command{
			Use:   "message TONE SECONDS TITLE : BODY",
			Short: "Put a message on the panel",
			Args:  cobra.MinimumNArgs(3),
		}, say),
		{
			Use:       "open settings|camera|player",
			Short:     "Open a settings page on the panel",
			Args:      cobra.ExactArgs(1),
			ValidArgs: []string{"settings", "camera", "player"},
			RunE: func(_ *cobra.Command, args []string) error {
				switch args[0] {
				case "settings":
					panel.Open()
				case "camera":
					panel.OpenCamera()
				case "player":
					media.Get().Open()
				default:
					return fmt.Errorf("open: %q is not a page", args[0])
				}
				return nil
			},
		},
		screencmd.Timer(timer.Get),
		{
			Use:   "clear",
			Short: "Take the message away again",
			Args:  cobra.NoArgs,
			RunE: func(_ *cobra.Command, _ []string) error {
				message.Get().Hide()
				return nil
			},
		},
		says(&cobra.Command{
			Use:   "stack",
			Short: "The screens on the shell's stack, top first, and which are held up",
			Args:  cobra.NoArgs,
		}, func([]string) (string, error) { return screencmd.Stack(shell.Get()), nil }),
	}
}

func telling() []*cobra.Command {
	return []*cobra.Command{
		says(&cobra.Command{
			Use:   "ready [MILLISECONDS]",
			Short: "Wait until the panel has settled",
			Args:  cobra.MaximumNArgs(1),
		}, ready),
		harness.WaitCommand(),
		says(&cobra.Command{
			Use:       "log [debug|info|warn|error]",
			Short:     "Read or change how much goes to logcat, until lanovod restarts",
			Args:      cobra.MaximumNArgs(1),
			ValidArgs: []string{"debug", "info", "warn", "error"},
		}, func(args []string) (string, error) { return harness.LogLevel(&logging.Log.Level, args) }),
		says(&cobra.Command{
			Use:   "peers",
			Short: "Other devices on the network",
			Args:  cobra.NoArgs,
		}, func([]string) (string, error) { return callcmd.Peers(discovery.Get().Peers()), nil }),
	}
}

func settingsCommand() *cobra.Command {
	return says(&cobra.Command{
		Use:   "settings [NAME [VALUE]]",
		Short: "Read or change a setting",
		Long: "With nothing after it, every setting and what it is. With a name, that one.\n" +
			"With a name and a value, that one changed.",
		Args: cobra.ArbitraryArgs,
	}, set)
}

func recording() *cobra.Command {
	rec := harness.ContextSays(&cobra.Command{
		Use:   "record SECONDS [PATH]",
		Short: "Record the levelled 16 kHz stream the wake word hears",
		Args:  cobra.RangeArgs(1, 2),
	}, record)
	rec.AddCommand(
		harness.ContextSays(&cobra.Command{
			Use:   "raw SECONDS [PATH]",
			Short: "Record the capture as read",
			Args:  cobra.RangeArgs(1, 2),
		}, recordRaw),
		harness.ContextSays(&cobra.Command{
			Use:   "echo SECONDS [PATH]",
			Short: "Record the microphones, the speaker feed and the cancelled microphones",
			Args:  cobra.RangeArgs(1, 2),
		}, recordEcho),
	)
	return rec
}

func sounding() []*cobra.Command {
	return []*cobra.Command{
		recording(),
		says(&cobra.Command{
			Use:   "player",
			Short: "What is playing, and on what",
			Args:  cobra.NoArgs,
		}, player),
		does(&cobra.Command{
			Use:   "volume STREAM LEVEL",
			Short: "Set a stream's level",
			Args:  cobra.ExactArgs(2),
		}, level),
		says(&cobra.Command{
			Use:   "mixer NAME [= VALUE]",
			Short: "Read or change a mixer control",
			Args:  cobra.MinimumNArgs(1),
		}, mixer),
	}
}

func verbOf(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
