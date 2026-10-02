package control

import (
	"fmt"
	"image/png"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/gpu"
	"github.com/ygelfand/LANovo/internal/ui/reveal"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func bootFrame(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("want FILE SECONDS")
	}
	secs, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return "", fmt.Errorf("seconds %q: %w", args[1], err)
	}
	m := reveal.Moment{At: time.Duration(secs * float64(time.Second))}
	nums := []*float64{&m.Trace, &m.Header, &m.Ready}
	w, h := board.Current().PanelWidth, board.Current().PanelHeight
	palette, ok := theme.ByName(config.Get().Screen.Theme)
	if !ok {
		palette = theme.Default()
	}
	n := 0
	for _, a := range args[2:] {
		if v, err := strconv.ParseFloat(a, 64); err == nil && n < len(nums) {
			*nums[n] = v
			n++
			continue
		}
		if x, y, found := strings.Cut(a, "x"); found {
			if w, err = strconv.Atoi(x); err != nil {
				return "", err
			}
			if h, err = strconv.Atoi(y); err != nil {
				return "", err
			}
			continue
		}
		if t, ok := theme.ByName(a); ok {
			palette = t
			continue
		}
		return "", fmt.Errorf("%q is not a number, a size or a theme", a)
	}

	l, err := gpu.OpenOffscreen(w, h)
	if err != nil {
		return "", err
	}
	defer l.Close()
	rv := reveal.New(palette)
	for i := range 3 {
		if err := rv.Shade(l, i == 0, w, h, m); err != nil {
			return "", err
		}
		time.Sleep(thumbStep)
	}
	img, err := l.Read()
	if err != nil {
		return "", err
	}
	f, err := os.Create(args[0])
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return "", err
	}
	return fmt.Sprintf("boot frame at %.2fs to %s", secs, args[0]), nil
}
