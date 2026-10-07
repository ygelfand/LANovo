package settings

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
)

func callPages() *sharedsettings.CallPages {
	return sharedsettings.NewCallPages(sharedsettings.CallDependencies{Settings: config.CallSection, Shell: shell.Get(), Controller: call.Get()})
}
