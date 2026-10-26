package lanovo

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/parts"
)

func newRCCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rc",
		Short: "Print lanovod's init service definition for a board",
		Long: "Prints the init rc lanovod runs under on --board. For a board whose files live\n" +
			"under /data, put it in the boot image's ramdisk at the path `lanovo check` names.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if boardName == "" {
				return errors.New("name the board with --board")
			}
			b, ok := board.ByName(boardName)
			if !ok {
				return fmt.Errorf("no board %q", boardName)
			}
			rc, err := parts.RC(b.Paths())
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(rc)
			return err
		},
	}
}
