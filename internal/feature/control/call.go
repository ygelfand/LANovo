package control

import (
	"github.com/spf13/cobra"
	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"
	sharedcmd "github.com/ygelfand/libcountertop/pkg/runtime/control/callcmd"

	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
)

func calling() []*cobra.Command {
	c := call.Get()
	return []*cobra.Command{
		harness.CallCommand(
			harness.CallActions{
				Dial:   dial,
				State:  callState,
				Answer: c.Answer,
				Hangup: c.Hangup,
				Mute:   c.Mute,
				Camera: c.Camera,
			},
		),
	}
}
func dial(a []string) (string, error) {
	return sharedcmd.Dial(call.Get(), discovery.Get().Peers(), a)
}
func callState(a []string) (string, error) { return sharedcmd.State(call.Get(), a) }
