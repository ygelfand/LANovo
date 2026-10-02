package lanovod

import (
	"log/slog"
	"os"

	"github.com/ygelfand/LANovo/internal/android/logd"
	"github.com/ygelfand/LANovo/internal/layout"
)

// openLog points slog at logcat: `logcat -s lanovod`.
//
// init points a service's stdout and stderr at /dev/null, so a boot-started service that only
// prints to stderr says nothing anyone can read. Lines carry the uptime, because the clock is
// wrong until something sets it.
func openLog() func() {
	h := logd.NewHandler(layout.LogTag, os.Stderr)
	slog.SetDefault(slog.New(h))
	return func() { h.Close() }
}
