package settings

import (
	"log/slog"

	"github.com/ygelfand/libcountertop/pkg/say"

	"github.com/ygelfand/LANovo/internal/feature/shell"

	"github.com/ygelfand/LANovo/internal/config"
)

func SetLanguage(tag string) {
	say.Use(tag)

	if err := config.Set().Screen().Language(tag); err != nil {
		slog.Error("saving a setting failed", "setting", "screen.language", "err", err)
	}
	shell.Get().Redraw()
}
