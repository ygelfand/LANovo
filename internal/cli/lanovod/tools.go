package lanovod

import "github.com/spf13/cobra"

func newToolsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "tools",
		Short: "Hardware diagnostics run on the device",
		Long: "Direct access to the display's audio and sensor hardware, for characterizing it\n" +
			"and for checking a unit after install.",
	}
	c.AddCommand(newMixerCmd(), newOrientCmd(), newProbeCmd(),
		newToneCmd(), newControlCmd(), newMTKAudioCmd())
	return c
}
