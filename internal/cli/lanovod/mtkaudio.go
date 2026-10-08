package lanovod

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"

	"github.com/spf13/cobra"

	"github.com/ygelfand/libcountertop/pkg/audio/alsa"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/mtkaudio"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func newMTKAudioCmd() *cobra.Command {
	var device int
	var switches []string
	var mic bool
	var listen float64
	var save string
	bits := 16
	capture := 1
	var hz, seconds, level float64
	channel := "both"

	c := &cobra.Command{
		Use:   "mtkaudio",
		Short: "Load the amplifier from the stock tables and play a tone through it",
		Long: "Sets the given mixer switches, plays silence on the device while the amplifier is\n" +
			"powered and loaded, then plays a tone and puts the amplifier back to sleep.\n\n" +
			"  lanovod tools mtkaudio --device 0\n" +
			"  lanovod tools mtkaudio --device 4 --switch 'O03 I05 Switch' --switch 'O04 I06 Switch'\n" +
			"  lanovod tools mtkaudio --mic       also bring up the microphone ADC\n" +
			"  lanovod tools mtkaudio --mic --seconds 0 --listen 5   record and print each channel's level\n" +
			"  lanovod tools mtkaudio --channel left|right|inverted",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(*cobra.Command, []string) error {
			b := board.Current()
			if det, err := board.Detect(prop.Local); err == nil {
				b = det
			}
			if b.SoC != board.MediaTek || b.Amp == nil {
				return errors.New("this board has no MediaTek amplifier driven from userspace")
			}

			t, err := mtkaudio.Stock()
			if err != nil {
				return err
			}
			fmt.Printf("tables: amp init %d, mic init %d\n", len(t.AmpInit), len(t.MicInit))

			if len(switches) > 0 {
				m, err := alsa.OpenMixer(speaker.Card)
				if err != nil {
					return err
				}
				for _, s := range switches {
					if err := m.SetInt(s, 1); err != nil {
						_ = m.Close()
						return fmt.Errorf("%s: %w", s, err)
					}
				}
				_ = m.Close()
			}

			if seconds == 0 {
				return micOn(b, t, mic, capture, listen, save, bits)
			}

			out, err := alsa.OpenPlayback(speaker.Card, device, alsa.Config{
				Rate: speaker.Rate, Channels: speaker.Channels, Format: alsa.FormatS16LE,
				Bits: speaker.Bits, PeriodSize: 960, Periods: 8,
			})
			if err != nil {
				return err
			}
			defer out.Close()

			stop, stopped := make(chan struct{}), make(chan error, 1)
			go func() {
				quiet := make([]byte, 960*speaker.FrameBytes)
				for {
					select {
					case <-stop:
						stopped <- nil
						return
					default:
					}
					if _, err := out.Write(quiet); err != nil {
						stopped <- err
						return
					}
				}
			}()

			err = mtkaudio.AmpOn(*b.Amp, t)
			close(stop)
			if werr := <-stopped; werr != nil {
				return fmt.Errorf("silence on device %d: %w", device, werr)
			}
			if err != nil {
				return err
			}
			fmt.Println("amplifier loaded")

			if err := micOn(b, t, mic, capture, listen, save, bits); err != nil {
				return err
			}

			fmt.Printf(
				"%.0f Hz at %.2f for %.1fs on device %d, %s\n",
				hz,
				level,
				seconds,
				device,
				channel,
			)
			if err := playOn(out, hz, seconds, level, channel); err != nil {
				return err
			}
			if err := out.Drain(); err != nil {
				return err
			}
			return mtkaudio.AmpPlay(*b.Amp, t, false)
		},
	}

	c.Flags().IntVar(&device, "device", 0, "pcm device")
	c.Flags().StringArrayVar(&switches, "switch", nil, "mixer switch to turn on, repeatable")
	c.Flags().BoolVar(&mic, "mic", false, "also bring up the microphone ADC")
	c.Flags().Float64Var(&listen, "listen", 0, "seconds to record from the capture device")
	c.Flags().IntVar(&capture, "capture", 1, "capture pcm device")
	c.Flags().StringVar(&save, "save", "", "file to write the raw capture to")
	c.Flags().IntVar(&bits, "bits", 16, "capture sample width, 16 or 32")
	c.Flags().Float64Var(&hz, "hz", 440, "frequency")
	c.Flags().Float64Var(&seconds, "seconds", 2, "how long to play")
	c.Flags().Float64Var(&level, "level", 0.1, "amplitude, nought to one")
	c.Flags().
		StringVar(&channel, "channel", "both", "both, left, right, or inverted (right is left negated)")
	return c
}

func micOn(
	b board.Board,
	t mtkaudio.Tables,
	mic bool,
	capture int,
	listen float64,
	save string,
	bits int,
) error {
	if mic && b.MicADC != nil {
		if err := mtkaudio.MicOn(*b.MicADC, t); err != nil {
			return err
		}
		fmt.Println("microphone ADC loaded")
	}
	if listen > 0 {
		return listenOn(capture, listen, save, bits)
	}
	return nil
}

func listenOn(device int, seconds float64, save string, bits int) error {
	format := alsa.FormatS16LE
	if bits == 32 {
		format = alsa.FormatS32LE
	}

	var raw *os.File
	if save != "" {
		f, err := os.Create(save)
		if err != nil {
			return err
		}
		defer f.Close()
		raw = f
	}

	for _, ch := range []int{2, 4, 8, 1} {
		in, err := alsa.Open(speaker.Card, device, alsa.Config{
			Rate: speaker.Rate, Channels: ch, Format: format,
			Bits: bits, PeriodSize: 960, Periods: 8,
		})
		if err != nil {
			fmt.Printf("capture %d at %d channels: %v\n", device, ch, err)
			continue
		}
		defer in.Close()
		fmt.Printf(
			"capture %d: %d channels of %d bits at %d Hz, %.1fs\n",
			device,
			ch,
			bits,
			speaker.Rate,
			seconds,
		)

		sum, peak := make([]float64, ch), make([]int, ch)
		same := true
		buf := make([]byte, 960*in.FrameBytes())
		for n := 0; n < int(seconds*speaker.Rate); {
			got, err := in.Read(buf)
			if err != nil {
				return err
			}
			if raw != nil {
				if _, err := raw.Write(buf[:got]); err != nil {
					return err
				}
			}
			for f := 0; f < got/in.FrameBytes(); f++ {
				first := sampleAt(buf, f*in.FrameBytes(), bits)
				for c := range ch {
					v := sampleAt(buf, f*in.FrameBytes()+c*bits/8, bits)
					sum[c] += float64(v) * float64(v)
					peak[c] = max(peak[c], abs16(v))
					same = same && v == first
				}
				n++
			}
		}
		frames := seconds * speaker.Rate
		for c := range ch {
			rms := math.Sqrt(sum[c] / frames)
			fmt.Printf(
				"  ch%d rms %.1f dBFS peak %.1f dBFS\n",
				c,
				dbfs(rms),
				dbfs(float64(peak[c])),
			)
		}
		fmt.Printf("  channels identical: %v\n", same)
		return nil
	}
	return errors.New("capture would not open at any channel count")
}

func sampleAt(buf []byte, at, bits int) int16 {
	if bits == 32 {
		return int16(binary.LittleEndian.Uint32(buf[at:]) >> 16)
	}
	return int16(binary.LittleEndian.Uint16(buf[at:]))
}

func abs16(v int16) int {
	if v < 0 {
		return -int(v)
	}
	return int(v)
}

func dbfs(v float64) float64 { return 20 * math.Log10(max(v, 1)/32768) }
