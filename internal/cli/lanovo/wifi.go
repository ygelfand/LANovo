package lanovo

import (
	"context"
	"io"

	"github.com/spf13/cobra"
	"github.com/ygelfand/libcountertop/pkg/host/adb"
	"github.com/ygelfand/libcountertop/pkg/host/wifisetup"

	"github.com/ygelfand/LANovo/internal/android/wifi"
	"github.com/ygelfand/LANovo/internal/host/takeover"
	"github.com/ygelfand/LANovo/internal/layout"
)

func newWifiCmd() *cobra.Command {
	var r wifisetup.Request

	c := &cobra.Command{
		Use:   "wifi",
		Short: "Connect the display to a wireless network",
		Long: "Scans, joins and reports. The display needs this before Home Assistant can reach it,\n" +
			"and its address is the identity Home Assistant keys the device on.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, out := cmd.Context(), cmd.OutOrStdout()

			d, err := connect(cmd.Context(), cmd.OutOrStdout())
			if err != nil {
				return err
			}
			b, err := takeover.DetectBoard(d, boardName)
			if err != nil {
				return err
			}
			return ensureWifi(ctx, out, d, b.Paths(), r)
		},
	}

	c.Flags().StringVar(&r.SSID, "ssid", "", "network to join, instead of picking from a scan")
	c.Flags().StringVar(&r.Password, "password", "", "passphrase, for an --ssid that needs one")
	c.Flags().BoolVar(&r.WPS, "wps", false, "join by pressing the router's WPS button instead")
	c.Flags().BoolVar(&r.Force, "force", false, "pick a network even when already connected")
	return c
}

func ensureWifi(
	ctx context.Context,
	out io.Writer,
	d *adb.Device,
	p layout.Paths,
	r wifisetup.Request,
) error {
	if err := wifi.Up(d, p); err != nil {
		return err
	}
	return wifisetup.Ensure(ctx, out, wifi.On(d, p), r)
}
