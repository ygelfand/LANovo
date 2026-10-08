package control

import (
	"github.com/spf13/cobra"
	sharedcmd "github.com/ygelfand/libcountertop/pkg/runtime/control/homecmd"

	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
)

func homeAssistant() []*cobra.Command { return sharedcmd.Commands(homeassistant.Get()) }
