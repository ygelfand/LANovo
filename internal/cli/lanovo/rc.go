package lanovo

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/parts"
)

func newRCCmd() *cobra.Command {
	var name string
	c := &cobra.Command{
		Use:   "rc",
		Short: "Print lanovod's init service definition for a board",
		Long: "Prints the init rc lanovod runs under on --board. For a board whose files live\n" +
			"under /data, put it in the boot image's ramdisk at the path `lanovo check` names.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			b, ok := board.ByName(name)
			if !ok {
				return fmt.Errorf("no board %q", name)
			}
			rc, err := parts.RC(b.Paths())
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(rc)
			return err
		},
	}
	c.Flags().StringVar(&name, "board", "", "board name, such as penghu")
	_ = c.MarkFlagRequired("board")
	return c
}
