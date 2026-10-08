package lanovod

import (
	"github.com/spf13/cobra"
	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"

	"github.com/ygelfand/LANovo/internal/feature/control"
)

var sequence = harness.Sequence

func newCtlCmd() *cobra.Command {
	return harness.ClientCommand(
		harness.ClientOptions{Name: "lanovod", Socket: control.Socket, Local: control.Local},
	)
}
