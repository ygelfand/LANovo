package lanovod

import (
	"log/slog"
	"os"

	"github.com/ygelfand/LANovo/internal/logging"
)

// init points a service's stdout and stderr at /dev/null.
func openLog() func() {
	h := logging.Log.Open(os.Stderr)
	slog.SetDefault(slog.New(h))
	return func() { _ = h.Close() }
}
