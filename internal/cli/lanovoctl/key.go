package lanovoctl

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/host/device"
	"github.com/ygelfand/LANovo/internal/host/takeover"
)

func newKeyCmd() *cobra.Command {
	var rotate bool

	c := &cobra.Command{
		Use:   "key",
		Short: "Show the encryption key Home Assistant pairs with",
		Long: "A device comes up unprovisioned and Home Assistant sets the key when it adds it.\n" +
			"This shows the key it set.\n\n" +
			"--rotate replaces it. Home Assistant holds the old one, so it will stop connecting\n" +
			"until the new key is put in.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := connect(cmd.Context(), cmd.OutOrStdout())
			if err != nil {
				return err
			}
			ctx, out := cmd.Context(), cmd.OutOrStdout()

			if rotate {
				if isTerminal() {
					ok, err := confirm(
						ctx,
						out,
						"Replace the key? Home Assistant will stop connecting until it is given the new one.",
					)
					if err != nil {
						return err
					}
					if !ok {
						return ErrCanceled
					}
				}

				key, err := takeover.RotateKey(d)
				if err != nil {
					return err
				}
				showKey(out, key)
				return nil
			}

			key, err := takeover.Key(d)
			if err != nil {
				return err
			}
			showKey(out, key)
			return nil
		},
	}

	c.Flags().BoolVar(&rotate, "rotate", false, "replace the key with a new one")
	return c
}

// showKey prints it on a line of its own.
//
// Not in the two column layout the rest of an install uses: a key is forty four characters, it is
// the one thing on the screen somebody has to copy exactly, and a column that wraps it is a key
// that gets copied wrong.
func showKey(out io.Writer, key string) {
	fmt.Fprintf(out, "\n%s\n", styleTitle.Render("Encryption key"))
	fmt.Fprintf(out, "  %s\n", key)
	fmt.Fprintf(
		out,
		"  %s\n",
		styleDetail.Render("Home Assistant asks for this when it adds the device."),
	)
}

// reportKey ends an install by saying how the device will be added.
//
// No key is the ordinary outcome, not a failure: the device comes up unprovisioned and Home
// Assistant sets a key when it adds it, which is what the code on the screen is for. generate is
// for an install that wants to choose the key itself instead.
func reportKey(ctx context.Context, out io.Writer, d *device.Device, generate bool) {
	key, err := takeover.Key(d)

	// A device that already has one keeps it, whatever was asked for. Home Assistant holds that
	// key, and replacing it during an install would break the pairing without saying so.
	if generate && errors.Is(err, takeover.ErrNoKey) {
		made, err := takeover.RotateKey(d)
		if err != nil {
			fmt.Fprintf(out, "  %-18s %s\n", "key", styleFail.Render(err.Error()))
			return
		}
		showKey(out, made)
		return
	}

	switch {
	case err == nil:
		showKey(out, key)

	case errors.Is(err, takeover.ErrNoKey):
		fmt.Fprintf(out, "\n%s\n", styleTitle.Render("Adding it"))
		fmt.Fprintf(out, "  %s\n", styleDetail.Render(
			"scan the code on the screen, and Home Assistant will set the key itself"))

	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return

	default:
		fmt.Fprintf(out, "  %-18s %s\n", "key", styleFail.Render(err.Error()))
	}
}
