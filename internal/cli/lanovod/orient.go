package lanovod

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/hardware/motion"
)

func newOrientCmd() *cobra.Command {
	var seconds int

	c := &cobra.Command{
		Use:   "orient",
		Short: "Print what the accelerometer says and which way up it makes the device",
		Long: "One line a second: the three axes in g, how far the total is from gravity, and the\n" +
			"rotation that follows. Turn the device while it runs and the axis signs can be read\n" +
			"off the hardware rather than guessed at.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, bus, err := motion.Open()
			if err != nil {
				return err
			}
			defer bus.Close()

			tracker := motion.NewTracker()
			out := cmd.OutOrStdout()

			fmt.Fprintf(out, "%-4s %8s %8s %8s %8s  %s\n", "at", "x", "y", "z", "|g|", "orientation")
			for i := range seconds {
				r, err := s.Read()
				if err != nil {
					return err
				}
				tracker.Update(r)

				fmt.Fprintf(out, "%-4d %8.3f %8.3f %8.3f %8.3f  %s\n",
					i, r.X, r.Y, r.Z, r.Magnitude(), tracker.Orientation())

				time.Sleep(time.Second)
			}
			return nil
		},
	}

	c.Flags().IntVar(&seconds, "seconds", 10, "how long to watch for")
	return c
}
