package lanovod

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ygelfand/libcountertop/pkg/audio/alsa"

	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func newMixerCmd() *cobra.Command {
	var values bool

	c := &cobra.Command{
		Use:   "mixer [word]",
		Short: "Print the sound card's mixer controls",
		Long: "The card has a couple of thousand controls and mixer_paths.xml describes only what\n" +
			"the vendor HAL replays, so this is how a route is found rather than guessed at.\n" +
			"A word narrows the list to controls whose name contains it.\n\n" +
			"With --values, each control is printed with what it is set to. That is how the state\n" +
			"of a card that is working gets captured, to diff against one that is not.",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var want string
			if len(args) > 0 {
				want = args[0]
			}

			if values {
				return dumpValues(cmd, want)
			}

			found, err := speaker.Controls(want)
			if err != nil {
				return err
			}

			for _, name := range found {
				fmt.Fprintln(cmd.OutOrStdout(), name)
			}
			fmt.Fprintf(os.Stderr, "%d control(s)\n", len(found))
			return nil
		},
	}

	c.Flags().BoolVar(&values, "values", false, "print what each control is set to")
	return c
}

func dumpValues(cmd *cobra.Command, want string) error {
	m, err := alsa.OpenMixer(speaker.Card)
	if err != nil {
		return err
	}
	defer m.Close()

	all, err := m.Controls()
	if err != nil {
		return err
	}

	var n int
	for _, ctl := range all {
		if want != "" && !strings.Contains(ctl.Name, want) {
			continue
		}
		n++

		v, err := m.Get(ctl)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t<unreadable>\n", ctl.Name)
			continue
		}

		said := make([]string, 0, len(v))
		for _, at := range v {
			if ctl.Type == alsa.TypeEnumerated && int(at) < len(ctl.Items) {
				said = append(said, ctl.Items[at])
				continue
			}
			said = append(said, strconv.FormatUint(uint64(at), 10))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", ctl.Name, strings.Join(said, " "))
	}

	fmt.Fprintf(os.Stderr, "%d control(s)\n", n)
	return nil
}
