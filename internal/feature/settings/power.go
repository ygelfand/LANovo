package settings

import (
	"log/slog"

	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
	sharedshell "github.com/ygelfand/libcountertop/pkg/display/shell"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/update"
)

func powerPage() *sharedshell.Page {
	return sharedsettings.PowerPage(shell.Get().Push, restartService, rebootDevice)
}

func restartService() { update.Restart("asked for on the panel") }

func rebootDevice() {
	safe.Go("reboot", func() {
		slog.Warn("rebooting, asked for on the panel")
		if err := prop.Reboot(prop.Local); err != nil {
			slog.Error("rebooting failed", "err", err)
		}
	})
}
