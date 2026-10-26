package settings

import (
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
	sharedshell "github.com/ygelfand/libcountertop/pkg/display/shell"
	"github.com/ygelfand/libcountertop/pkg/display/syncview"

	"github.com/ygelfand/LANovo/internal/feature/access"
	"github.com/ygelfand/LANovo/internal/feature/alerts"
	"github.com/ygelfand/LANovo/internal/feature/chromecast"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/firmware"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/lock"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/network"
	"github.com/ygelfand/LANovo/internal/feature/photos"
	"github.com/ygelfand/LANovo/internal/feature/poster"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
)

func Visuals() *sharedsettings.BasicPages { return basicPages() }

func basicPages() *sharedsettings.BasicPages {
	return sharedsettings.NewBasicPages(
		sharedsettings.BasicDependencies{
			Preferences: preferences(),
			Shell:       shell.Get(),
			Screen:      screen.Get(),
			Poster:      poster.Get(),
			Photos:      photos.Get(),
			Visuals:     visuals.Get(),
			Engine:      chromecast.Tuning(),
			Privacy:     privacy.Get(),
			Access:      access.Get(),
			Zone:        clock.Get(),
			Dashboard:   dashboard.Get(),
			Clocks:      clocks,
			Idle:        idlePages(),
			UISize:      uiSize,
			Home:        homecontrol.Get(),
			HA:          homeassistant.Get(),
			Marks:       modeMarks(),
			Alerts:      alerts.Get(),
			Updates:     firmware.Get(),
			Lock:        lock.Get().Gate(),
			HomeKey:     network.Get(),
			Media:       media.Get(),
			Previews:    previewKit(),
			PowerPage:   func() sharedshell.View { return powerPage() },
			SyncPage:    func() sharedshell.View { return &syncview.Directory{} },
			VisualPage:  func() sharedshell.View { return visualPage() },
		},
	)
}
