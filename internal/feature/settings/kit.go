package settings

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/display/previews"
	"github.com/ygelfand/libcountertop/pkg/display/theme"
	"github.com/ygelfand/libcountertop/pkg/display/widgets"
	schema "github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/config"
)

var previewKit = sync.OnceValue(func() previews.Kit {
	ui := widgets.NewUI(config.ScreenSection)
	return previews.NewKit(ui,
		func() bool { return config.Get().Screen.Hours == schema.TwentyFourHour },
		func() theme.Theme { return config.Get().Clock.Ink.Over(ui.Palette()) },
	)
})
