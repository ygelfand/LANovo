package lanovod

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/libcountertop/pkg/network/wpa"
)

// scanFor is how long a scan is given. Results arrive over several seconds and the scan reads until
// the list stops growing, so this only has to be longer than that.
const scanFor = 20 * time.Second

func newWifiCmd() *cobra.Command {
	var join, passphrase, forget string
	var scan bool

	cmd := &cobra.Command{
		Use:   "wifi",
		Short: "Scan, join and forget networks from the device",
		Long: "Talks to wpa_supplicant over its control socket, which is how the device changes\n" +
			"its own network with no cable in it.\n\n" +
			"Joining keeps the network the device is on and goes back to it when the new one does\n" +
			"not associate, so a wrong passphrase does not strand the device. Nothing is saved\n" +
			"unless the join worked.\n\n" +
			"With no flags it reports the connection and the networks already configured.\n\n" +
			"  lanovod wifi\n" +
			"  lanovod wifi --scan\n" +
			"  lanovod wifi --join Kitchen --passphrase 'a good one'\n" +
			"  lanovod wifi --forget Kitchen",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if scan {
				within, stop := context.WithTimeout(ctx, scanFor)
				defer stop()

				found, err := wifi.Scan(within)
				if err != nil {
					return err
				}
				for _, n := range found {
					fmt.Println(" ", n)
				}
			}

			switch {
			case forget != "":
				if err := wifi.Forget(forget); err != nil {
					return err
				}
				fmt.Printf("forgot %q\n", forget)

			case join != "":
				within, stop := context.WithTimeout(ctx, wpa.Settle+10*time.Second)
				defer stop()

				if err := wifi.Join(within, join, passphrase); err != nil {
					return err
				}
				fmt.Printf("joined %q\n", join)
			}

			return report()
		},
	}

	cmd.Flags().BoolVar(&scan, "scan", false, "list what the radio can see")
	cmd.Flags().StringVar(&join, "join", "", "move to this network")
	cmd.Flags().StringVar(&passphrase, "passphrase", "", "its passphrase, empty for an open network")
	cmd.Flags().StringVar(&forget, "forget", "", "remove this network, unless it is the one in use")
	return cmd
}

// report is the connection and what is configured, which is what says whether anything worked.
func report() error {
	c, err := wifi.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	status, err := c.Status()
	if err != nil {
		return err
	}
	fmt.Printf("%s on %s\n", status["wpa_state"], status["ssid"])

	known, err := wifi.Networks()
	if err != nil {
		return err
	}
	fmt.Println("configured:", known)
	return nil
}
