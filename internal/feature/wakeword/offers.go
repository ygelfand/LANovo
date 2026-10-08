package wakeword

import (
	"log/slog"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/lib/wake"
)

// Home Assistant's question carries the models in its custom_wake_words directory, each with a URL.
func Answer(offered []esphome.ExternalWakeWord) []esphome.WakeWord {
	lib := wake.Lib()

	lib.Offered(offered)
	words, shadowed := lib.Advertise()

	slog.Debug("wake words offered", "count", len(offered),
		"ours", len(lib.Ours()), "advertised", len(words), "shadowed", shadowed)
	return words
}
