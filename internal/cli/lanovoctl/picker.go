package lanovoctl

import (
	"context"
	"fmt"
	"io"

	"github.com/ygelfand/LANovo/internal/host/device"
)

// resolveSerial decides which device to act on.
//
// An explicit --serial always wins. On a terminal the person picks, even when only one device is
// attached, so it is clear what is about to be written to. Off a terminal there is nobody to ask,
// so the choice is left to device.Connect, which takes the only one or says there are several.
func resolveSerial(ctx context.Context, out io.Writer, want string) (string, error) {
	if want != "" {
		return want, nil
	}
	if !isTerminal() {
		return "", nil
	}

	devices, err := device.List()
	if err != nil {
		return "", err
	}
	if len(devices) == 0 {
		return "", device.ErrUnreachable
	}
	return pickDevice(ctx, out, devices)
}

func pickDevice(ctx context.Context, out io.Writer, devices []device.Info) (string, error) {
	chosen, err := choose(ctx, out, "Select a device", devices,
		func(d device.Info) string { return fmt.Sprintf("%s  %s", d, d.Serial) }, "")
	if err != nil {
		return "", err
	}
	return chosen.Serial, nil
}
