package lanovod

import (
	"github.com/spf13/cobra"
	"github.com/ygelfand/LANovo/internal/feature/control"
	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"
)

var sequence = harness.Sequence

func newCtlCmd() *cobra.Command {
	return harness.ClientCommand(
		harness.ClientOptions{Name: "lanovod", Socket: control.Socket, Local: control.Local},
	)
}
