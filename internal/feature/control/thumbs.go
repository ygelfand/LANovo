package control

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ygelfand/libcountertop/pkg/audio/analysis"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/gpu"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

const (
	DefaultThumbs = "/data/local/tmp/lanovo-thumbs"
	thumbFrames   = 50
	thumbStep     = 40 * time.Millisecond
	beatEvery     = 500 * time.Millisecond
)

var thumbShapes = []struct {
	suffix       string
	w, h, tw, th int
}{
	{"", board.Blueberry.PanelHeight, board.Blueberry.PanelWidth, 384, 240},
	{"-portrait", board.Blueberry.PanelWidth, board.Blueberry.PanelHeight, 240, 384},
}

func thumbs(args []string) (string, error) {
	dir := DefaultThumbs
	if len(args) > 0 {
		dir = args[0]
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	n := 0
	for _, k := range visual.Built() {
		for _, sh := range thumbShapes {
			img, err := renderThumb(k, sh.w, sh.h, thumbFrames)
			if err != nil {
				return "", fmt.Errorf("%s %dx%d: %w", k, sh.w, sh.h, err)
			}
			f, err := os.Create(filepath.Join(dir, string(k)+sh.suffix+".jpg"))
			if err != nil {
				return "", err
			}
			err = jpeg.Encode(f, shrink(img, sh.tw, sh.th), &jpeg.Options{Quality: 82})
			f.Close()
			if err != nil {
				return "", err
			}
			n++
		}
	}
	return fmt.Sprintf("wrote %d thumbnails to %s", n, dir), nil
}

func renderThumb(k visual.Kind, w, h, frames int) (*image.RGBA, error) {
	l, err := gpu.Get().OpenOffscreen(w, h)
	if err != nil {
		return nil, err
	}
	defer l.Close()
	v := visual.New(k)
	x := visual.Input{Mic: loud(), Speaker: loud(), Dt: thumbStep, Label: "LANOVO"}
	for i := range frames {
		x.Now = time.Duration(i+1) * thumbStep
		x.Replying = i >= frames/2
		if err := v.Shade(l, i == 0, ui.Rect{W: w, H: h}, x); err != nil {
			return nil, err
		}
		time.Sleep(thumbStep)
	}
	return l.Read()
}

func loud() analysis.Analysis {
	var a analysis.Analysis
	a.Level, a.Peak, a.Onsets = 0.4, 0.7, 3
	for b := range a.Bands {
		a.Bands[b] = float32(0.35 + 0.3*math.Sin(float64(b)*0.55) + 0.25*math.Cos(float64(b)*0.21))
	}
	for i := range a.Wave {
		p := 2 * math.Pi * float64(i) / float64(len(a.Wave))
		a.Wave[i] = float32(0.55*math.Sin(3*p) + 0.3*math.Sin(7*p+1) + 0.1*math.Sin(13*p))
	}
	return a
}

func shrink(src *image.RGBA, tw, th int) *image.RGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	out := image.NewRGBA(image.Rect(0, 0, tw, th))
	for y := range th {
		y0, y1 := y*h/th, (y+1)*h/th
		for x := range tw {
			x0, x1 := x*w/tw, (x+1)*w/tw
			var r, g, b, n int
			for sy := y0; sy < y1; sy++ {
				row := src.Pix[sy*src.Stride:]
				for sx := x0; sx < x1; sx++ {
					r, g, b, n = r+int(row[sx*4]), g+int(row[sx*4+1]), b+int(row[sx*4+2]), n+1
				}
			}
			o := out.PixOffset(x, y)
			out.Pix[o], out.Pix[o+1], out.Pix[o+2], out.Pix[o+3] = uint8(
				r/n,
			), uint8(
				g/n,
			), uint8(
				b/n,
			), 255
		}
	}
	return out
}

func render(args []string) (string, error) {
	w, h, frames := board.Blueberry.PanelWidth, board.Blueberry.PanelHeight, thumbFrames
	var err error
	if len(args) >= 4 {
		if w, err = strconv.Atoi(args[2]); err != nil {
			return "", err
		}
		if h, err = strconv.Atoi(args[3]); err != nil {
			return "", err
		}
	}
	if len(args) >= 5 {
		if frames, err = strconv.Atoi(args[4]); err != nil {
			return "", err
		}
	}
	img, err := renderThumb(visual.Kind(args[0]), w, h, frames)
	if err != nil {
		return "", err
	}
	f, err := os.Create(args[1])
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return "", err
	}
	return fmt.Sprintf("rendered %s at %dx%d to %s", args[0], w, h, args[1]), nil
}

func clip(args []string) (string, error) {
	w, h, seconds, fps := 480, 300, 3, 12
	nums := []*int{&w, &h, &seconds, &fps}
	for i, a := range args[2:] {
		n, err := strconv.Atoi(a)
		if err != nil || n <= 0 {
			return "", fmt.Errorf("want positive numbers for W H SECONDS FPS")
		}
		*nums[i] = n
	}
	if err := os.MkdirAll(args[1], 0o755); err != nil {
		return "", err
	}
	rw, rh := board.Blueberry.PanelHeight, board.Blueberry.PanelWidth
	if h > w {
		rw, rh = rh, rw
	}
	l, err := gpu.Get().OpenOffscreen(rw, rh)
	if err != nil {
		return "", err
	}
	defer l.Close()
	v := visual.New(visual.Kind(args[0]))
	step := time.Second / time.Duration(fps)
	frames := seconds * fps
	x := visual.Input{Dt: step, Label: "LANOVO"}
	for i := range thumbFrames + frames {
		x.Now = time.Duration(i+1) * step
		x.Mic, x.Speaker = beat(x.Now), beat(x.Now+step/2)
		x.Replying = i >= thumbFrames+frames/2
		if err := v.Shade(l, i == 0, ui.Rect{W: rw, H: rh}, x); err != nil {
			return "", err
		}
		if i < thumbFrames {
			continue
		}
		img, err := l.Read()
		if err != nil {
			return "", err
		}
		f, err := os.Create(filepath.Join(args[1], fmt.Sprintf("%03d.png", i-thumbFrames)))
		if err != nil {
			return "", err
		}
		err = png.Encode(f, shrink(img, w, h))
		f.Close()
		if err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("wrote %d frames of %s at %dx%d to %s", frames, args[0], w, h, args[1]), nil
}

func beat(at time.Duration) analysis.Analysis {
	t := at.Seconds()
	phase := math.Mod(t, beatEvery.Seconds())
	kick := math.Exp(-phase / 0.12)
	var a analysis.Analysis
	a.Level = float32(0.12 + 0.55*kick)
	a.Peak = float32(min(1, 0.3+0.7*kick))
	if phase < 0.05 {
		a.Onsets = 1
	}
	for b := range a.Bands {
		low := math.Exp(-float64(b) / 6)
		a.Bands[b] = float32(0.15 + 0.6*kick*low + 0.25*(0.5+0.5*math.Sin(t*2.1+float64(b)*0.5)))
	}
	for i := range a.Wave {
		p := 2 * math.Pi * float64(i) / float64(len(a.Wave))
		a.Wave[i] = float32((0.25 + 0.6*kick) * (0.6*math.Sin(3*p+t*4) + 0.3*math.Sin(7*p+t*9)))
	}
	return a
}
