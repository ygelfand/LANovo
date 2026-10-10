package lanovo

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/ygelfand/libcountertop/pkg/host/adb"
	"github.com/ygelfand/libcountertop/pkg/host/prompt"

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
				if prompt.IsTerminal() {
					ok, err := prompt.Confirm(
						ctx,
						out,
						"Replace the key? Home Assistant will stop connecting until it is given the new one.",
					)
					if err != nil {
						return err
					}
					if !ok {
						return prompt.ErrCanceled
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

func showKey(out io.Writer, key string) {
	fmt.Fprintf(out, "\n%s\n", styleTitle.Render("Encryption key"))
	fmt.Fprintf(out, "  %s\n", key)
	fmt.Fprintf(
		out,
		"  %s\n",
		styleDetail.Render("Home Assistant asks for this when it adds the device."),
	)
}

func reportKey(ctx context.Context, out io.Writer, d *adb.Device, generate bool) {
	key, err := takeover.Key(d)

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
