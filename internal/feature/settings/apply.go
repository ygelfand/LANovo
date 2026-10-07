package settings

import (
	"log/slog"

	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/libcountertop/pkg/say"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/screen"
)

// What touching a control actually does. Each goes through whatever owns the setting rather than
// to the hardware, so Home Assistant is told as well.

func setBrightness(level int) { screen.Get().SetBacklight(level) }

func toggleAuto(auto bool) {
	want := config.ModeAuto
	if auto {
		want = config.ModeManual
	}
	screen.Get().SetMode(want)
}

func toggleHours(now config.HourFormat) {
	want := config.TwentyFourHour
	if now == config.TwentyFourHour {
		want = config.TwelveHour
	}
	dashboard.Get().SetHours(want)
}

// SetLanguage changes the text and remembers it.
//
// Everything on the panel is rebuilt rather than this page alone: a Build closure resolves its
// strings when it runs, so a screen that is not rebuilt keeps the language it was drawn in.
func SetLanguage(tag string) {
	say.Use(tag)

	if err := config.Set().Screen().Language(tag); err != nil {
		slog.Error("saving a setting failed", "setting", "screen.language", "err", err)
	}
	shell.Get().Redraw()
}
