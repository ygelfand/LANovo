package control

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/ble"
)

// radio asks the Bluetooth chip things the stack has no opinion about.
//
//	ble sniff [ms]   everything the line carries for a while
func radio(args []string) (string, error) {
	if len(args) == 0 || args[0] != "sniff" {
		return "", fmt.Errorf("ble: %q is not one of its commands", verbOf(args))
	}

	within := 2 * time.Second
	if len(args) > 1 {
		ms, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("ble: %q is not a number of milliseconds", args[1])
		}
		within = time.Duration(ms) * time.Millisecond
	}

	seen, err := ble.Get().Sniff(within)
	if err != nil {
		return "", err
	}
	if len(seen) == 0 {
		return "the line carried nothing at all\n", nil
	}
	return strings.Join(seen, "\n") + "\n", nil
}
