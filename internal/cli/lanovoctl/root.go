package lanovoctl

import (
	"os"

	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"
	"github.com/ygelfand/libcountertop/pkg/host/prompt"
)

var serial string

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "lanovoctl",
		Short: "LANovo host CLI",
		Long: "lanovoctl installs LANovo on a Lenovo Smart Display and manages the parts of\n" +
			"Android that would otherwise fight it.",
		SilenceUsage: true,
	}

	root.PersistentFlags().
		StringVar(&serial, "serial", "", "device to act on, when more than one is attached")

	root.AddCommand(newCheckCmd())
	root.AddCommand(newInstallCmd())
	root.AddCommand(newWifiCmd())
	root.AddCommand(newKeyCmd())
	return root
}

func Execute() {
	root := newRoot()
	if !prompt.IsTerminal() {
		root.SetOut(colorprofile.NewWriter(os.Stdout, os.Environ()))
		root.SetErr(colorprofile.NewWriter(os.Stderr, os.Environ()))
	}

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
