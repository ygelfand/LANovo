package lanovod

import (
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"

	"github.com/ygelfand/libcountertop/pkg/audio/alsa"

	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

const fade = 20 * time.Millisecond

func newToneCmd() *cobra.Command {
	var hz float64
	var seconds, level float64
	var route bool
	channel := "both"
	device := speaker.PlaybackDevice

	c := &cobra.Command{
		Use:   "tone",
		Short: "Play a tone straight at the card",
		Long: "Opens the playback device and writes a sine. Nothing else: no service, no mixer\n" +
			"changes unless asked, no arbitration.\n\n" +
			"This is how the playback path is told apart from the routing. Against a card some\n" +
			"other system has already set up and is making sound with, a tone that is heard says\n" +
			"our writing is fine and only our routing is wrong; a tone that is not says the fault\n" +
			"is in how we open or feed the card.\n\n" +
			"  lanovod tools tone\n" +
			"  lanovod tools tone --route        also apply our own speaker path first\n" +
			"  lanovod tools tone --hz 1000 --seconds 5 --level 0.5",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if route {
				if err := speaker.Route(); err != nil {
					return err
				}
				fmt.Println("applied our speaker path")
			}

			out, err := alsa.OpenPlayback(speaker.Card, device, alsa.Config{
				Rate:       speaker.Rate,
				Channels:   speaker.Channels,
				Format:     alsa.FormatS16LE,
				Bits:       speaker.Bits,
				PeriodSize: 960,
				Periods:    8,
			})
			if err != nil {
				return err
			}
			defer out.Close()

			fmt.Printf("%.0f Hz at %.2f for %.1fs on card %d device %d\n",
				hz, level, seconds, speaker.Card, device)

			if err := playOn(out, hz, seconds, level, channel); err != nil {
				return err
			}
			return out.Drain()
		},
	}

	c.Flags().Float64Var(&hz, "hz", 440, "frequency")
	c.Flags().Float64Var(&seconds, "seconds", 3, "how long to play")
	c.Flags().Float64Var(&level, "level", 0.3, "amplitude, nought to one")
	c.Flags().BoolVar(&route, "route", false, "apply our speaker path before playing")
	c.Flags().
		StringVar(&channel, "channel", "both", "both, left, right, or inverted (right is left negated)")
	c.Flags().
		IntVar(&device, "device", speaker.PlaybackDevice, "pcm device: 0 is MultiMedia1, 1 is MultiMedia2")
	return c
}

func channelGains(which string) ([speaker.Channels]float64, error) {
	switch which {
	case "both":
		return [speaker.Channels]float64{1, 1}, nil
	case "left":
		return [speaker.Channels]float64{1, 0}, nil
	case "right":
		return [speaker.Channels]float64{0, 1}, nil
	case "inverted":
		return [speaker.Channels]float64{1, -1}, nil
	}
	return [speaker.Channels]float64{}, fmt.Errorf(
		"channel %q: want both, left, right or inverted",
		which,
	)
}

func playOn(out *alsa.Playback, hz, seconds, level float64, which string) error {
	const period = 960

	gains, err := channelGains(which)
	if err != nil {
		return err
	}

	total := int(seconds * speaker.Rate)
	rise := int(fade.Seconds() * speaker.Rate)

	buf := make([]byte, period*speaker.FrameBytes)
	for at := 0; at < total; at += period {
		for i := range period {
			n := at + i

			amp := level
			switch {
			case n < rise:
				amp *= float64(n) / float64(rise)
			case n > total-rise:
				amp *= float64(total-n) / float64(rise)
			}

			s := amp * math.MaxInt16 * math.Sin(2*math.Pi*hz*float64(n)/speaker.Rate)
			for ch := range speaker.Channels {
				v := int16(s * gains[ch])
				at := (i*speaker.Channels + ch) * 2
				buf[at], buf[at+1] = byte(v), byte(v>>8)
			}
		}

		if _, err := out.Write(buf); err != nil {
			return err
		}
	}
	return nil
}
