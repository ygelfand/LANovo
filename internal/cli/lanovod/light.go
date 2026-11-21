package lanovod

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/hardware/light"
)

func newLightCmd() *cobra.Command {
	var (
		seconds int
		pulses  int
		led     int
	)

	c := &cobra.Command{
		Use:   "light",
		Short: "Print what the light sensor reads: lux and raw proximity counts",
		Long: "Four lines a second. Move a hand toward the sensor while it runs to see how far\n" +
			"proximity rises over its resting count.\n\n" +
			"  lanovod tools light --seconds 20\n" +
			"  lanovod tools light --pulses 64 --led 0x37   LTR-578 only; lasts until lanovod restarts",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, bus, err := light.Open()
			if err != nil {
				return err
			}
			defer bus.Close()

			if pulses >= 0 || led >= 0 {
				ltr, ok := s.(*light.LTR578)
				if !ok {
					return errors.New("--pulses and --led apply to the LTR-578 only")
				}
				if pulses < 0 {
					pulses = 16
				}
				if led < 0 {
					led = 0x36
				}
				if err := ltr.Emitter(byte(pulses), byte(led)); err != nil {
					return err
				}
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%-6s %10s %10s\n", "at", "lux", "proximity")
			start := time.Now()
			for time.Since(start) < time.Duration(seconds)*time.Second {
				lux, err := s.Lux()
				if err != nil {
					return err
				}
				near, err := s.Proximity()
				if err != nil {
					return err
				}
				fmt.Fprintf(out, "%-6.2f %10.1f %10d\n", time.Since(start).Seconds(), lux, near)
				time.Sleep(250 * time.Millisecond)
			}
			return nil
		},
	}

	c.Flags().IntVar(&seconds, "seconds", 10, "how long to watch for")
	c.Flags().IntVar(&pulses, "pulses", -1, "LTR-578 proximity pulses per reading")
	c.Flags().IntVar(&led, "led", -1, "LTR-578 PS_LED register: pulse frequency and current")
	return c
}
