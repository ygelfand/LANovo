package control

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"

	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/message"
	panel "github.com/ygelfand/LANovo/internal/feature/settings"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

// What the harness can do, in one place.
//
// This was a switch on the first argument with a help string written out beside it by hand, and the
// two drifted the way they always do — the help is the part nobody remembers to change. Here a
// command carries its own, so there is one description of it and it cannot go stale.
//
// Built per invocation rather than once. Commands hold parsed state, and these arrive down a socket
// from whoever is poking at the device, not from a process that starts and exits.

// tree is every command the socket takes.
func (c *Control) tree() *cobra.Command {
	root := newRoot()
	root.AddCommand(harness.DeviceCommands(deviceInfo)...)
	root.AddCommand(c.touching()...)
	root.AddCommand(c.showing()...)
	root.AddCommand(sounding()...)
	root.AddCommand(radios()...)
	root.AddCommand(watching()...)
	root.AddCommand(casting()...)
	root.AddCommand(homeAssistant()...)
	root.AddCommand(calling()...)
	harness.Organize(root)
	return root
}

func Local(args []string) (string, error) {
	return harness.Execute(context.Background(), func() *cobra.Command {
		root := harness.NewRoot()
		root.AddCommand(casting()...)
		harness.Organize(root)
		return root
	}, args)
}

var newRoot = harness.NewRoot
var literal = harness.Literal
var says = harness.Says
var group = harness.Group
var does = harness.Does

// touching is the commands that pretend to be a finger.
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

// showing is the commands about what is on the panel.
func (c *Control) showing() []*cobra.Command {
	return []*cobra.Command{
		says(&cobra.Command{
			Use:   "shot [PATH]",
			Short: "Take a screenshot",
			Args:  cobra.MaximumNArgs(1),
		}, shot),
		harness.ContextSays(&cobra.Command{
			Use:   "record [raw|echo] SECONDS [PATH]",
			Short: "Record the microphones for a while",
			Args:  cobra.RangeArgs(1, 3),
		}, record),
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
		{
			Use:   "timer SECONDS [NAME] | timer ring | timer cancel",
			Short: "Count a timer down on the panel, ring it, or cancel it",
			Args:  cobra.MinimumNArgs(1),
			RunE:  countdown,
		},
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
			Use:   "ready [MILLISECONDS]",
			Short: "Wait until the panel has settled",
			Args:  cobra.MaximumNArgs(1),
		}, ready),
		does(&cobra.Command{
			Use:   "wait MILLISECONDS",
			Short: "Do nothing for a while",
			Args:  cobra.ExactArgs(1),
		}, pause),
		says(&cobra.Command{
			Use:   "settings [NAME [VALUE]]",
			Short: "Read or change a setting",
			Long: "With nothing after it, every setting and what it is. With a name, that one.\n" +
				"With a name and a value, that one changed.",
			Args: cobra.ArbitraryArgs,
		}, set),
		says(&cobra.Command{
			Use:       "log [debug|info|warn|error]",
			Short:     "Read or change how much goes to logcat, until lanovod restarts",
			Args:      cobra.MaximumNArgs(1),
			ValidArgs: []string{"debug", "info", "warn", "error"},
		}, logLevel),
		says(&cobra.Command{
			Use:   "peers",
			Short: "Other devices on the network",
			Args:  cobra.NoArgs,
		}, peers),
	}
}

// sounding is the commands about noise.
func sounding() []*cobra.Command {
	return []*cobra.Command{
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

// verbOf names what was asked for, for the guards inside the handlers that still dispatch on their
// own first argument.
//
// Those guards cannot be reached through the tree — cobra turns an unknown subcommand away before a
// handler sees it, and the tree always supplies a known verb. They stay as a guard against being
// called some other way, but they no longer list their siblings: a list written out by hand beside
// the thing it describes is the drift this was all meant to end.
func verbOf(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
