package control

import (
	"fmt"
	"strconv"
	"time"

	"github.com/ygelfand/libcountertop/pkg/display/panel"

	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

const (
	readyWait  = 90 * time.Second
	readyEvery = 200 * time.Millisecond
)

func ready(args []string) (string, error) {
	wait := readyWait
	if len(args) > 0 {
		ms, err := strconv.Atoi(args[0])
		if err != nil || ms < 0 {
			return "", fmt.Errorf("milliseconds must be a number")
		}
		wait = time.Duration(ms) * time.Millisecond
	}

	deadline := time.Now().Add(wait)
	for display.Get().Held(panel.PriorityBoot) {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("still starting after %s", wait)
		}
		time.Sleep(readyEvery)
	}
	if shell.Get().Open() {
		return "screen", nil
	}
	return "dashboard", nil
}
