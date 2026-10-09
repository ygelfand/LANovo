package control

import (
	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
)

func set(args []string) (string, error) { return harness.Settings(config.Get, settings(), args) }

func settings() []harness.Setting[config.Config] {
	return harness.Registered[config.Config](&component.Settings)
}
