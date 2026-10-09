package board

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const (
	ModesPath  = "/sys/class/graphics/fb0/modes"
	InputsPath = "/proc/bus/input/devices"
)

type Reader interface {
	ReadFile(path string) ([]byte, error)
}

type Facts struct {
	Width, Height int
	Inputs        []string
}

func Read(r Reader) (Facts, error) {
	modes, err := r.ReadFile(ModesPath)
	if err != nil {
		return Facts{}, fmt.Errorf("board: %w", err)
	}
	w, h, ok := parseMode(string(modes))
	if !ok {
		return Facts{}, fmt.Errorf("board: no panel size in %s: %q", ModesPath, modes)
	}
	inputs, err := r.ReadFile(InputsPath)
	if err != nil {
		return Facts{}, fmt.Errorf("board: %w", err)
	}
	return Facts{Width: w, Height: h, Inputs: parseInputs(string(inputs))}, nil
}

func parseMode(s string) (int, int, bool) {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	if _, rest, ok := strings.Cut(line, ":"); ok {
		line = rest
	}
	size, _, _ := strings.Cut(line, "p")
	ws, hs, ok := strings.Cut(size, "x")
	w, e1 := strconv.Atoi(ws)
	h, e2 := strconv.Atoi(hs)
	if !ok || e1 != nil || e2 != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

func parseInputs(s string) []string {
	var names []string
	for line := range strings.Lines(s) {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "N: Name=")
		if ok {
			names = append(names, strings.Trim(rest, `"`))
		}
	}
	return names
}

func (b Board) fits(f Facts) bool {
	return b.PanelWidth == f.Width && b.PanelHeight == f.Height &&
		slices.Contains(f.Inputs, b.Touch)
}

func Detect(f Facts) (Board, error) {
	var found []Board
	for _, b := range boards {
		if b.fits(f) {
			found = append(found, b)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return Board{}, fmt.Errorf(
			"board: no board has a %dx%d panel and touch among %q",
			f.Width,
			f.Height,
			f.Inputs,
		)
	}
	names := make([]string, len(found))
	for i, b := range found {
		names[i] = b.Name
	}
	return Board{}, fmt.Errorf(
		"board: %dx%d with %q fits %s",
		f.Width,
		f.Height,
		f.Inputs,
		strings.Join(names, ", "),
	)
}
