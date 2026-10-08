package control

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

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
