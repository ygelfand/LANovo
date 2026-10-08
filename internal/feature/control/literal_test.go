package control

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"

	"github.com/ygelfand/LANovo/internal/config"
)

func TestEveryCommandTakesItsArgumentsAsWritten(t *testing.T) {
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if !cmd.DisableFlagParsing {
			t.Errorf("%s parses flags, so a value like -1 is refused", cmd.CommandPath())
		}
		for _, sub := range cmd.Commands() {
			walk(sub)
		}
	}
	walk(build("").tree())
}

func TestANegativeValueReachesTheCommand(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))

	_, err := harness.Execute(
		context.Background(),
		build("").tree,
		[]string{"device", "wait", "-1"},
	)
	if err != nil && strings.Contains(err.Error(), "flag") {
		t.Errorf("-1 was read as a flag: %v", err)
	}
}
