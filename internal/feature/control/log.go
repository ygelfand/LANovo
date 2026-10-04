package control

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ygelfand/LANovo/internal/android/logd"
)

func logLevel(args []string) (string, error) {
	if len(args) == 1 {
		var l slog.Level
		if err := l.UnmarshalText([]byte(args[0])); err != nil {
			return "", err
		}
		logd.SetThreshold(l)
		slog.Log(context.Background(), max(l, slog.LevelInfo), "log level", "level", l)
	}
	return strings.ToLower(logd.Threshold().String()), nil
}
