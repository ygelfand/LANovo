package control

import (
	"github.com/ygelfand/libcountertop/pkg/runtime/control/screencmd"

	"github.com/ygelfand/LANovo/internal/hardware/display"
)

const DefaultShot = "/data/misc/lanovo/screen.png"

func shot(args []string) (string, error) {
	path := DefaultShot
	if len(args) > 0 {
		path = args[0]
	}
	if err := screencmd.Shot(display.Get(), path); err != nil {
		return "", err
	}
	return path, nil
}
