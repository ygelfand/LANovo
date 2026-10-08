package lanovoctl

import (
	"context"
	"fmt"
	"io"

	"github.com/ygelfand/LANovo/internal/host/device"
)

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
