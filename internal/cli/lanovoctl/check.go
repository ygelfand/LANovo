package lanovoctl

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/host/takeover"
)

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Report whether a display is ready for LANovo",
		Long: "Reads the device and says what is and is not in order. Unlocking the bootloader\n" +
			"and flashing the debug firmware happen with fastboot before LANovo is involved,\n" +
			"so those are checked rather than done.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := connect(cmd.Context(), cmd.OutOrStdout())
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			checks, err := takeover.Preflight(d)
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "%s\n", styleTitle.Render(d.Serial()))
			for _, c := range checks {
				fmt.Fprintf(out, "  %-18s %s  %s\n", c.Name, mark(c.OK), c.Got)
				if !c.OK && c.Fix != "" {
					fmt.Fprintf(out, "  %-18s    %s\n", "", styleDetail.Render(c.Fix))
				}
			}

			if blocked := takeover.Blocked(checks); len(blocked) > 0 {
				fmt.Fprintf(out, "\n%s\n", styleFail.Render("not ready"))
				return fmt.Errorf("%d prerequisite(s) not met", len(blocked))
			}

			fmt.Fprintf(out, "\n%s\n", styleDone.Render("ready"))
			return nil
		},
	}
}
