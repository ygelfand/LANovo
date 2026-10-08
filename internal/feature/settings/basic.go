package settings

import (
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"

	"github.com/ygelfand/LANovo/internal/feature/access"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/poster"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
)

func basicPages() *sharedsettings.BasicPages {
	return sharedsettings.NewBasicPages(
		sharedsettings.BasicDependencies{
			Preferences:  preferences(),
			Shell:        shell.Get(),
			Screen:       screen.Get(),
			Poster:       poster.Get(),
			Visuals:      visuals.Get(),
			Privacy:      privacy.Get(),
			Access:       access.Get(),
			Zone:         clock.Get(),
			Dashboard:    dashboard.Get(),
			Clocks:       clocks,
			Idle:         idlePages(),
			UISize:       uiSize,
			PowerPage:    func() shell.View { return powerPage() },
			VisualPage:   func() shell.View { return visualPage() },
			PresencePage: func() shell.View { return sensorsPage() },
		},
	)
}
