package control

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

// turn faces the picture a different way without anybody picking the device up.
//
// Which way up it is stands behind a lot of the drawing — a face lays out differently, the dock
// moves, the logo goes to a corner — and until now the only way to see any of that was to reach
// over and turn it. The accelerometer still wins the moment the device actually moves, so this
// cannot leave the panel stuck facing a way it is not.
func turn(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("want mounted|portrait|landscape|left|right|0|90|180|270")
	}

	rot, ok := facing(args[0])
	if !ok {
		return fmt.Errorf("no such orientation %q", args[0])
	}

	sensors.Get().Turn(rot)
	return nil
}

// facing reads an orientation as a name or as the degrees it is.
//
// Names because that is how somebody at a terminal thinks about it, and degrees because that is
// what the display calls them and a harness that could only say the names could not reach 180.
func facing(s string) (display.Orientation, bool) {
	switch strings.ToLower(s) {
	case "mounted":
		return display.Mounted(), true
	case "portrait", "up":
		return display.Rotate0, true
	case "landscape", "right":
		return display.Rotate90, true
	case "down", "upside":
		return display.Rotate180, true
	case "left":
		return display.Rotate270, true
	}

	deg, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}

	switch display.Orientation(deg) {
	case display.Rotate0, display.Rotate90, display.Rotate180, display.Rotate270:
		return display.Orientation(deg), true
	}
	return 0, false
}
