package lanovod

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/control"
)

// Turning the control socket on and off from the device itself.
//
// The socket is what `lanovod ctl` talks to, so it cannot be the thing that turns it on: a device
// with it off and no Home Assistant attached would have no way back. This writes the setting
// directly, which is the same thing the switch does.
func newControlCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "control [on|off]",
		Short: "Whether the control socket listens",
		Long: "The control socket synthesises touches and writes screenshots for anything on the\n" +
			"device that can reach it, with no pairing and nothing recorded. It is off unless\n" +
			"asked for.\n\n" +
			"Takes effect when lanovod next starts, or immediately from Home Assistant's switch.\n\n" +
			"  lanovod tools control          say whether it is on\n" +
			"  lanovod tools control on\n" +
			"  lanovod tools control off",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				fmt.Printf("%-16s %s\n", control.Socket, onOff(control.Enabled()))
				return nil
			}

			var on bool
			switch args[0] {
			case "on":
				on = true
			case "off":
			default:
				return fmt.Errorf("%q is not on or off", args[0])
			}

			if err := config.Set().Access().Control(on); err != nil {
				return fmt.Errorf("saving it: %w", err)
			}

			fmt.Printf("%-16s %s\n", control.Socket, onOff(on))
			return nil
		},
	}
	return c
}

func onOff(on bool) string {
	if on {
		return "listening once lanovod restarts"
	}
	return "closed"
}
