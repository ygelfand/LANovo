package control

import (
	"github.com/spf13/cobra"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	sharedcmd "github.com/ygelfand/libcountertop/pkg/runtime/control/homecmd"
)

func homeAssistant() []*cobra.Command { return sharedcmd.Commands(homeassistant.Get()) }
