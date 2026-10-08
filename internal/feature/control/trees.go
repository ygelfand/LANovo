package control

import (
	"github.com/spf13/cobra"
	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/btcmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/cameracmd"

	"github.com/ygelfand/LANovo/internal/feature/a2dp"
	"github.com/ygelfand/LANovo/internal/feature/vision"
)

func radios() []*cobra.Command {
	bt := btcmd.Command(a2dp.Get(), "lanovod")

	ble := group("ble", "The low energy radio", "")
	ble.AddCommand(says(&cobra.Command{
		Use:   "sniff [MILLISECONDS]",
		Short: "Listen for advertisements",
		Args:  cobra.MaximumNArgs(1),
	}, harness.Under(radio, "sniff")))

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
		}, func(args []string) (string, error) { return cameracmd.Still(vision.Get().Still, args) }),
	)
	return camera
}
