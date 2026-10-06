package update

import (
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
)

var restartWait = 10 * time.Second

// Restart ends lanovod so init starts it again. SIGTERM to ourselves takes the clean shutdown that
// paints the restarting screen; init's ctl.restart is SIGKILL to the process group.
func Restart(why string) {
	safe.Go("restart", func() {
		slog.Warn("restarting", "why", why)

		if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
			slog.Error("could not signal ourselves, leaving it to init", "err", err)
			hand()
			return
		}

		time.Sleep(restartWait)
		slog.Error("the shutdown did not finish, leaving it to init")
		hand()
	})
}

func hand() {
	if err := prop.Restart(prop.Local, layout.Service); err != nil {
		slog.Error("restarting failed", "err", err)
	}
}
