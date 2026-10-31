package control

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	sharedapps "github.com/ygelfand/libcountertop/pkg/display/apps"
	"github.com/ygelfand/libcountertop/pkg/inference/asr"
	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/alarmcmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/castcmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/displaycmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/doorbellcmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/firmwarecmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/homealarmcmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/i2ccmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/mixercmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/playercmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/screencmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/speechcmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/thermostatcmd"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/alarm"
	"github.com/ygelfand/LANovo/internal/feature/apps"
	"github.com/ygelfand/LANovo/internal/feature/chromecast"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/doorbell"
	"github.com/ygelfand/LANovo/internal/feature/firmware"
	"github.com/ygelfand/LANovo/internal/feature/homealarm"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/message"
	"github.com/ygelfand/LANovo/internal/feature/nws"
	"github.com/ygelfand/LANovo/internal/feature/poster"
	panel "github.com/ygelfand/LANovo/internal/feature/settings"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/thermostat"
	"github.com/ygelfand/LANovo/internal/feature/timer"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/gpu"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/logging"
	"github.com/ygelfand/LANovo/internal/parts"
)

func (c *Control) tree() *cobra.Command {
	t := harness.NewTree()
	t.Add(harness.DeviceTask, harness.DeviceCommands(deviceInfo)...)
	t.Add(harness.DeviceTask, telling()...)
	t.Add(harness.DeviceTask, alarmcmd.Command(alarm.Get))
	t.Add(harness.InputTask, c.touching()...)
	t.Add(harness.DisplayTask, showing()...)
	t.Add(harness.AudioTask, sounding()...)
	t.Top(settingsCommand(), camera())
	t.Top(firmwarecmd.Command(firmware.Get))
	t.Top(radios()...)
	t.Top(homeAssistant()...)
	t.Top(calling()...)
	t.Top(castcmd.Command(chromecast.Page))
	t.Top(doorbellcmd.Command(doorbell.Get))
	t.Top(homealarmcmd.Command(homealarm.Get, homealarm.Shown))
	t.Top(thermostatcmd.Command(thermostat.Get, thermostat.Shown))
	return t.Root()
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
	shared := displaycmd.Commands(displaycmd.Dependencies{
		Panel:    display.Get,
		GPU:      gpu.Get,
		Messages: message.Get,
		Label:    config.Defaults().Visual.Label,
		Shot:     layout.StateDir + "/screen.png",
		Thumbs:   "/data/local/tmp/lanovo-thumbs",
	})
	return append(shared,
		screencmd.Record(display.Get),
		says(&cobra.Command{
			Use:   "boot FILE SECONDS [TRACE HEADER READY] [THEME] [WxH]",
			Short: "Render one frame of the boot reveal offscreen into a PNG",
			Args:  cobra.RangeArgs(2, 7),
		}, bootFrame),
		&cobra.Command{
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
		screencmd.Warning(nws.Get),
		screencmd.Wallpaper(poster.Get),
		screencmd.App(func() []sharedapps.App { return apps.Get().List() }),
		says(&cobra.Command{
			Use:   "stack",
			Short: "The screens on the shell's stack, top first, and which are held up",
			Args:  cobra.NoArgs,
		}, func([]string) (string, error) { return screencmd.Stack(shell.Get()), nil }),
	)
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
		harness.PeersCommand(discovery.Get().Peers),
		i2ccmd.Command(),
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
		playercmd.Command(func() playercmd.Player { return media.Get() }),
		does(&cobra.Command{
			Use:   "volume STREAM LEVEL",
			Short: "Set a stream's level",
			Args:  cobra.ExactArgs(2),
		}, level),
		mixercmd.Command(speaker.Card),
		speechcmd.Command(func(ctx context.Context) (asr.Options, error) {
			return asr.Prepare(ctx, layout.SpeechHelper, parts.Speech(), layout.ModelDir)
		}),
	}
}
