package lanovod

import (
	"log/slog"
	"os"

	"github.com/ygelfand/LANovo/internal/android/logd"
	"github.com/ygelfand/LANovo/internal/layout"
)

// init points a service's stdout and stderr at /dev/null.
func openLog() func() {
	h := logd.NewHandler(layout.LogTag, os.Stderr)
	slog.SetDefault(slog.New(h))
	return func() { _ = h.Close() }
}
