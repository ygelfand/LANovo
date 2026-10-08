package control

import (
	"github.com/spf13/cobra"
	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"
)

func radios() []*cobra.Command {
	bt := harness.BluetoothCommand(sink)

	ble := group("ble", "The low energy radio", "")
	ble.AddCommand(says(&cobra.Command{
		Use:   "sniff [MILLISECONDS]",
		Short: "Listen for advertisements",
		Args:  cobra.MaximumNArgs(1),
	}, under(radio, "sniff")))

	return []*cobra.Command{bt, ble}
}

func camera() *cobra.Command {
	camera := group(
		"camera",
		"The imaging hardware",
		"The vendor camera stack, through lanovo-camera.",
	)
	camera.AddCommand(
		says(&cobra.Command{
			Use:   "still [PATH]",
			Short: "Take the picture Home Assistant would get",
			Long: "Through the same component that answers the camera entity, so this is the whole\n" +
				"path and not a rehearsal of it. Writes to /data/local/tmp/still.jpg unless told\n" +
				"somewhere else.",
			Args: cobra.MaximumNArgs(1),
		}, still),
	)
	return camera
}

func under(do func([]string) (string, error), verb string) func([]string) (string, error) {
	return func(args []string) (string, error) {
		return do(append([]string{verb}, args...))
	}
}
