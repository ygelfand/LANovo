package lanovod

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/hardware/wifi"
)

func newWPACmd() *cobra.Command {
	return &cobra.Command{
		Use:   "wpa COMMAND [ARG...]",
		Short: "Send one command to wpa_supplicant and print its reply",
		Long: "Takes the same commands as wpa_cli.\n\n" +
			"  lanovod wpa ping\n" +
			"  lanovod wpa list_networks",
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := wifi.Get().Dial()
			if err != nil {
				return err
			}
			defer c.Close()

			words := append([]string{strings.ToUpper(args[0])}, args[1:]...)
			out, err := c.Cmd(strings.Join(words, " "))
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), out)
			return err
		},
	}
}
