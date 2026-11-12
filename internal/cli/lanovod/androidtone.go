package lanovod

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func newAndroidToneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "android-tone HELPER",
		Short: "Play a diagnostic tone through the LANovo mixer and Android AudioTrack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			var output speaker.Speaker
			output.AndroidPCM(args[0])
			output.SetVolume(0.5)
			output.SetLevel(schema.StreamMedia, 100)

			done := make(chan error, 1)
			go func() { done <- output.Run(ctx) }()
			ticker := time.NewTicker(20 * time.Millisecond)
			defer ticker.Stop()
			timeout := time.NewTimer(8 * time.Second)
			defer timeout.Stop()
			for output.Written() == 0 {
				select {
				case err := <-done:
					return fmt.Errorf("Android PCM stopped before playback: %v", err)
				case <-ctx.Done():
					return ctx.Err()
				case <-timeout.C:
					return fmt.Errorf("Android PCM did not start")
				case <-ticker.C:
				}
			}

			const frames = 3 * speaker.Rate
			samples := make([]int16, frames*speaker.Channels)
			for frame := range frames {
				value := int16(4000 * math.Sin(2*math.Pi*440*float64(frame)/speaker.Rate))
				for channel := range speaker.Channels {
					samples[frame*speaker.Channels+channel] = value
				}
			}
			end := output.Written() + frames + speaker.Rate/2
			output.Play(samples)
			for output.Written() < end {
				select {
				case err := <-done:
					return fmt.Errorf("Android PCM stopped during playback: %v", err)
				case <-ctx.Done():
					return ctx.Err()
				case <-timeout.C:
					return fmt.Errorf("Android PCM playback stalled")
				case <-ticker.C:
				}
			}
			cancel()
			return <-done
		},
	}
}
