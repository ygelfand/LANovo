package control

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ygelfand/LANovo/internal/hardware/i2c"
)

func i2cCommand() *cobra.Command {
	return says(&cobra.Command{
		Use:   "i2c [BUS...]",
		Short: "Every address that answers on each I2C bus, and the kernel driver holding it",
		Args:  cobra.ArbitraryArgs,
	}, scanI2C)
}

func scanI2C(args []string) (string, error) {
	buses, err := i2cBuses(args)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, n := range buses {
		bus, err := i2c.Open(n)
		if err != nil {
			fmt.Fprintf(&b, "bus %d: %v\n", n, err)
			continue
		}
		found := i2c.Scan(bus)
		_ = bus.Close()
		if len(found) == 0 {
			fmt.Fprintf(&b, "bus %d: nothing answered\n", n)
			continue
		}
		for _, addr := range found {
			fmt.Fprintf(&b, "bus %d: %#02x %s\n", n, addr, i2cDriver(n, addr))
		}
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

func i2cBuses(args []string) ([]int, error) {
	if len(args) > 0 {
		out := make([]int, 0, len(args))
		for _, a := range args {
			n, err := strconv.Atoi(a)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("i2c: %q is not a bus number", a)
			}
			out = append(out, n)
		}
		return out, nil
	}
	paths, _ := filepath.Glob("/dev/i2c-*")
	var out []int
	for _, p := range paths {
		if n, err := strconv.Atoi(strings.TrimPrefix(p, "/dev/i2c-")); err == nil {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("i2c: no buses under /dev")
	}
	slices.Sort(out)
	return out, nil
}

func i2cDriver(bus int, addr uint16) string {
	name, err := os.ReadFile(fmt.Sprintf("/sys/bus/i2c/devices/%d-%04x/name", bus, addr))
	if err != nil {
		return ""
	}
	return "(" + strings.TrimSpace(string(name)) + ")"
}
