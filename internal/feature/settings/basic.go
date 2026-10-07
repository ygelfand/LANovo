package settings

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/access"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	"github.com/ygelfand/LANovo/internal/feature/poster"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
)

func basicPages() *sharedsettings.BasicPages {
	return sharedsettings.NewBasicPages(sharedsettings.BasicOptions{
		Poster:         func() config.Poster { return config.Get().Poster },
		PosterEnabled:  poster.Get().SetEnabled,
		PosterNext:     poster.Get().Next,
		SetPosterEvery: poster.Get().SetEvery,
		SetDock:        screen.Get().SetDrawer,
		SetVolumeEdge:  func(e config.Edge) { useVolumeEdge(e)(0) },
		SetKeyboard:    SetKeyboard,
		ToggleHours:    toggleHours,
		PowerPage:      func() shell.View { return powerPage() },
		Read: func() sharedsettings.BasicState {
			c := config.Get()
			return sharedsettings.BasicState{
				Screen:      c.Screen,
				Clock:       c.Clock,
				Idle:        c.Idle,
				Visual:      c.Visual,
				PosterLabel: posterSays(c.Poster),
				UISize:      uiSize(),
			}
		},
		Push:         shell.Get().Push,
		ADB:          access.Get().ADB,
		SetADB:       access.Get().SetADB,
		SetFPS:       visuals.Get().SetMaxFPS,
		SetSeed:      visuals.Get().SetSeed,
		SetStyle:     func(s string) error { return screen.Get().Set("style", s) },
		SetTheme:     func(s string) { use(s)(0) },
		SetLanguage:  SetLanguage,
		VisualPage:   func() shell.View { return visualPage() },
		ClockPage:    func() shell.View { return clockPage() },
		IdlePage:     func() shell.View { return idlePage() },
		PosterPage:   func() shell.View { return posterPage() },
		DockPage:     func() shell.View { return edgePage() },
		VolumePage:   func() shell.View { return volumeEdgePage() },
		Brightness:   setBrightness,
		Auto:         toggleAuto,
		Size:         setSize,
		Marks:        privacy.Get().SetMarks,
		Zone:         clock.Get().Zone,
		SetZone:      clock.Get().SetZone,
		PresencePage: func() shell.View { return sensorsPage() },
	})
}
