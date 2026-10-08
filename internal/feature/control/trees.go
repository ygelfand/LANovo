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

func watching() []*cobra.Command {
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

	return []*cobra.Command{
		camera,
		says(&cobra.Command{
			Use:   "player",
			Short: "What is playing, and on what",
			Args:  cobra.NoArgs,
		}, player),
		says(&cobra.Command{
			Use:   "stack",
			Short: "The screens on the shell's stack, top first, and which are held up",
			Args:  cobra.NoArgs,
		}, stack),
	}
}

func under(do func([]string) (string, error), verb string) func([]string) (string, error) {
	return func(args []string) (string, error) {
		return do(append([]string{verb}, args...))
	}
}
