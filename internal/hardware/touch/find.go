package touch

import (
	"fmt"
	"os"
	"strings"
)

// devices is where the kernel lists what it has, and which event node each one is behind.
const devices = "/proc/bus/input/devices"

// Find is the event device a driver is behind. Looked up by name rather than fixed at event1,
// which is only true as long as nothing else registers first.
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

// handlerFor reads the blocks the kernel separates with a blank line, each naming a device on an
// N: line and its handlers on an H: line.
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
