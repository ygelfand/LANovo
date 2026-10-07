package settings

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/chromecast"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	sharedcast "github.com/ygelfand/libcountertop/pkg/display/settings/castpages"
)

func servicePages() *sharedcast.Pages {
	return sharedcast.New(sharedcast.Dependencies{Settings: config.CastSection, Shell: shell.Get(), Receiver: chromecast.Get()})
}
