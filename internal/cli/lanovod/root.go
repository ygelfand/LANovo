package lanovod

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/boot"
	"github.com/ygelfand/LANovo/internal/layout"
)

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "lanovod",
		Short: "LANovo device agent",
		Long: "lanovod runs on the Smart Display and presents it to Home Assistant over the\n" +
			"ESPHome native API. The tools subtree exposes the same hardware access for\n" +
			"diagnostics.",
		SilenceUsage: true,
		Version:      layout.VersionString(),
	}

	root.AddCommand(newRunCmd(), newToolsCmd(), newCtlCmd(), newBleCmd(), newWifiCmd())
	return root
}

func Execute() {
	if len(os.Args) == 1 {
		os.Args = append(os.Args, "run")
	}
	err := newRoot().Execute()
	if err == nil {
		return
	}

	var taken *boot.Taken
	if errors.As(err, &taken) {
		os.Exit(boot.ExitTaken)
	}
	os.Exit(1)
}
