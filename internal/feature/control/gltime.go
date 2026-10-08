package control

import (
	"fmt"
	"image"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ygelfand/libcountertop/pkg/display/ui"
	"github.com/ygelfand/libcountertop/pkg/display/visual"

	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/gpu"
)

func gltime(args []string) (string, error) {
	src, err := os.ReadFile(args[0])
	if err != nil {
		return "", err
	}
	nums := make([]float64, 5)
	for i, a := range args[1:] {
		if nums[i], err = strconv.ParseFloat(a, 64); err != nil {
			return "", fmt.Errorf("%q: %w", a, err)
		}
	}
	flags, amount, radius, passes, seconds := visual.Passes(
		nums[0],
	), float32(
		nums[1],
	), float32(
		nums[2],
	), int(
		nums[3],
	), nums[4]
	if seconds <= 0 {
		seconds = 5
	}
	w, h := display.Get().Native()
	l, err := gpu.Get().Open(w, h)
	if err != nil {
		return "", err
	}
	defer l.Close()
	if err := l.Place(ui.Rect{W: w, H: h}); err != nil {
		return "", err
	}
	noise := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range noise.Pix {
		noise.Pix[i] = byte(uint32(i) * 2654435761 >> 13)
	}
	for unit := range 4 {
		if err := l.Texture(unit, noise); err != nil {
			return "", err
		}
	}
	if err := l.Program(string(src), flags); err != nil {
		return "", err
	}
	u := make([]float32, 128)
	fixed := false
	if b, err := os.ReadFile(args[0] + ".u"); err == nil {
		for i, w := range strings.Fields(strings.Trim(string(b), "[] \n")) {
			v, err := strconv.ParseFloat(w, 32)
			if err != nil || i >= len(u) {
				return "", fmt.Errorf("%s.u: bad value %q", args[0], w)
			}
			u[i] = float32(v)
		}
		fixed = true
	}
	start := time.Now()
	t := time.NewTicker(time.Second / 60)
	defer t.Stop()
	frames := 0
	for now := range t.C {
		el := now.Sub(start).Seconds()
		if el > seconds {
			break
		}
		if !fixed {
			u[0] = float32(el)
		}
		if err := l.Values(u, amount, radius, passes); err != nil {
			return "", err
		}
		frames++
	}
	return fmt.Sprintf("sent %d frames of %dx%d", frames, w, h), nil
}
