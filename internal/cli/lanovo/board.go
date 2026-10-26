package lanovo

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/host/takeover"
)

func newBoardCmd() *cobra.Command {
	var sh bool

	c := &cobra.Command{
		Use:   "board",
		Short: "Say what board a device is, and where lanovod is installed there",
		Long: "The Makefile's device targets read this.\n\n" +
			"  lanovo board\n" +
			"  eval \"$(lanovo board --sh)\"",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, err := connect(cmd.Context(), cmd.OutOrStdout())
			if err != nil {
				return err
			}
			b, err := takeover.DetectBoard(d, boardName)
			if err != nil {
				return err
			}
			p := b.Paths()

			out := cmd.OutOrStdout()
			if sh {
				_, err := fmt.Fprintf(
					out,
					"board=%s\nroot=%s\nbinary=%s\n",
					b.Name,
					p.Root,
					p.Binary,
				)
				return err
			}
			fmt.Fprintf(out, "  %s %s\n", styleTitle.Render("board"), b.Name)
			fmt.Fprintf(out, "  %s %s\n", styleTitle.Render("root"), p.Root)
			fmt.Fprintf(out, "  %s %s\n", styleTitle.Render("binary"), p.Binary)
			return nil
		},
	}

	c.Flags().BoolVar(&sh, "sh", false, "print shell assignments instead")
	return c
}
