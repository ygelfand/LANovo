package touch

import (
	"fmt"
	"os"
	"strings"
)

const devices = "/proc/bus/input/devices"

func Find(name string) (string, error) {
	b, err := os.ReadFile(devices)
	if err != nil {
		return "", fmt.Errorf("touch: %w", err)
	}

	handler, ok := handlerFor(string(b), name)
	if !ok {
		return "", fmt.Errorf("touch: no input device called %q", name)
	}
	return "/dev/input/" + handler, nil
}

// /proc/bus/input/devices: blank-line separated blocks, N: names the device, H: its handlers.
func handlerFor(list, name string) (handler string, ok bool) {
	want := `N: Name="` + name + `"`

	for _, block := range strings.Split(list, "\n\n") {
		var named bool
		var handlers string

		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			switch {
			case line == want:
				named = true
			case strings.HasPrefix(line, "H: Handlers="):
				handlers = strings.TrimPrefix(line, "H: Handlers=")
			}
		}
		if !named {
			continue
		}

		for _, h := range strings.Fields(handlers) {
			if strings.HasPrefix(h, "event") {
				return h, true
			}
		}
	}
	return "", false
}
