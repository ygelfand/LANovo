package lanovod

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/hardware/ble"
)

func newBleCmd() *cobra.Command {
	var load, off, dump, active bool
	var scan time.Duration

	cmd := &cobra.Command{
		Use:   "ble",
		Short: "Bring up the Bluetooth radio",
		Long: "Powers the radio and asks the chip what it is.\n\n" +
			"There is no hci interface on this device and there cannot be one — the kernel has\n" +
			"CONFIG_BT without CONFIG_BT_HCIUART — so the chip is driven over its UART from here\n" +
			"the way Android's vendor service does it.\n\n" +
			"On its own this only powers the chip and reads its version, which is the cheap half\n" +
			"and says whether it is listening at all. With --load it goes on to push the patch and\n" +
			"the NVM, which is a hundred and thirty eight commands and the part that either works\n" +
			"or leaves the radio needing a power cycle.\n\n" +
			"  lanovod ble\n" +
			"  lanovod ble --load\n" +
			"  lanovod ble --off",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if off {
				changed, err := ble.Power(false)
				if err != nil {
					return err
				}
				fmt.Printf("radio off, changed=%v\n", changed)
				return nil
			}

			if load || dump || scan > 0 {
				ble.Listen = dump

				p, v, err := ble.BringUp()
				if err != nil {
					return err
				}
				defer p.Close()

				if dump {
					said, err := p.Raw(3 * time.Second)
					if err != nil {
						return err
					}
					fmt.Printf(
						"%s\nafter the download, %d bytes:\n%s",
						v,
						len(said),
						hex.Dump(said),
					)
					return nil
				}

				fmt.Printf("%s\nfirmware loaded\n", v)

				if scan == 0 {
					return nil
				}

				ctx, done := context.WithTimeout(cmd.Context(), scan)
				defer done()

				seen := map[uint64]int{}
				err = ble.Scan(ctx, ble.Reader(ctx, p), active, func(a ble.Advertisement) {
					if seen[a.Addr()] == 0 {
						fmt.Printf("%x  %4d dBm  % x\n", a.Address, a.RSSI, a.Data)
					}
					seen[a.Addr()]++
				})
				if err != nil {
					return err
				}

				fmt.Printf("%d devices, %d advertisements in %v\n", len(seen), total(seen), scan)
				return nil
			}

			changed, err := ble.Power(true)
			if err != nil {
				return err
			}
			fmt.Printf("radio on, changed=%v\n", changed)

			p, err := ble.Open(ble.TTY)
			if err != nil {
				return err
			}
			defer p.Close()

			if err := p.Flush(); err != nil {
				return err
			}

			v, err := ble.Ask(p)
			if err != nil {
				return fmt.Errorf("reading the version: %w", err)
			}
			fmt.Println(v)
			return nil
		},
	}

	cmd.Flags().
		BoolVar(&load, "load", false, "download the patch and NVM after reading the version")
	cmd.Flags().BoolVar(&off, "off", false, "block the radio again")
	cmd.Flags().
		BoolVar(&dump, "dump", false, "download, then print what the chip says instead of resetting it")
	cmd.Flags().
		DurationVar(&scan, "scan", 0, "after the bring-up, scan for advertisements for this long")
	cmd.Flags().
		BoolVar(&active, "active", false, "ask for scan responses rather than only listening")
	return cmd
}

func total(seen map[uint64]int) int {
	n := 0
	for _, count := range seen {
		n += count
	}
	return n
}
