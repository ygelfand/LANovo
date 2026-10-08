package lanovoctl

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/ygelfand/libcountertop/pkg/host/adb"
	"github.com/ygelfand/libcountertop/pkg/host/prompt"

	"github.com/ygelfand/LANovo/internal/host/takeover"
)

func resolveName(
	ctx context.Context,
	out io.Writer,
	d *adb.Device,
	flag string,
) (string, error) {
	existing, err := takeover.ReadName(d)
	if err != nil {
		return "", err
	}

	if flag != "" {
		if existing != "" && existing != flag {
			fmt.Fprintf(out, "%s\n", styleDetail.Render(fmt.Sprintf(
				"renaming %s to %s: Home Assistant will see a new device and keep the old one",
				existing, flag)))
		}
		return flag, takeover.ValidName(flag)
	}
	if existing != "" {
		return existing, nil
	}
	if !prompt.IsTerminal() {
		return "", fmt.Errorf("%w: pass --name to name this device", takeover.ErrNoName)
	}

	fmt.Fprintf(out, "%s\n", styleDetail.Render(
		"Home Assistant keys the device on its name, so changing it later creates a new one."))

	for {
		name, err := prompt.Line(ctx, out, "Name this device", takeover.SuggestName(d), false)
		if err != nil {
			return "", err
		}
		if err := takeover.ValidName(name); err != nil {
			fmt.Fprintf(out, "%s %v\n", styleFail.Render("✗"), err)
			continue
		}
		return name, nil
	}
}

func nameFlag(c *cobra.Command, target *string) {
	c.Flags().StringVar(target, "name", "", "device name Home Assistant sees; asked for if unset")
}
