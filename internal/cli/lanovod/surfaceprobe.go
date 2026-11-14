package lanovod

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/ygelfand/libcountertop/pkg/display/surface"

	"github.com/ygelfand/LANovo/internal/layout"
)

func newSurfaceProbeCmd() *cobra.Command {
	var draw bool
	var socket string
	cmd := &cobra.Command{
		Use:   "surface-probe",
		Short: "Check the display helper and optionally show a small test layer",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := surface.Dial(socket)
			if err != nil {
				return err
			}
			defer client.Close()
			fmt.Fprintf(cmd.OutOrStdout(), "Surface helper replied: %dx%d\n", client.Width, client.Height)
			if !draw {
				return nil
			}
			const id, size, z = 1, 120, 1_000_000
			layer, err := client.Create(id, 40, 40, size, size, z, 0)
			if err != nil {
				return err
			}
			defer client.Destroy(id)
			for y := range size {
				for x := range size {
					i := y*layer.Stride + x*4
					copy(layer.Pixels[i:i+4], []byte{0, 220, 80, 255})
				}
			}
			if err := client.Frame(id, 1, []surface.Rect{{W: size, H: size}}); err != nil {
				return err
			}
			if err := client.Scene([]surface.Placement{{
				ID: id, X: 40, Y: 40, W: size, H: size, Z: z, Visible: true, Alpha: 1,
			}}); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Surface helper accepted test frame")
			select {
			case <-cmd.Context().Done():
				return cmd.Context().Err()
			case <-time.After(4 * time.Second):
				return nil
			}
		},
	}
	cmd.Flags().BoolVar(&draw, "draw", false, "show a green square for four seconds")
	cmd.Flags().StringVar(&socket, "socket", layout.SurfaceSocket, "display helper socket")
	return cmd
}
