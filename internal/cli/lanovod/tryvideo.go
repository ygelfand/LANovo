package lanovod

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/video"
)

const videoFrame = time.Second / 30

type fileFrames struct {
	units [][]byte
	next  int
}

func (f *fileFrames) Next() (video.Frame, error) {
	if f.next == len(f.units) {
		return video.Frame{}, io.EOF
	}
	fr := video.Frame{Data: f.units[f.next], At: time.Duration(f.next) * videoFrame}
	f.next++
	return fr, nil
}

func wallClock() video.Clock {
	var began time.Time
	return func() (time.Duration, bool) {
		if began.IsZero() {
			began = time.Now()
		}
		return time.Since(began), true
	}
}

func tryVideo(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	codec, w, h, units := video.Split(data)
	if len(units) == 0 {
		return fmt.Errorf("no frames in %s", path)
	}
	began := time.Now()
	rep, err := video.Play(context.Background(), video.Stream{
		Codec: codec, Width: w, Height: h,
		Source: &fileFrames{units: units},
		Clock:  wallClock(),
	})
	took := time.Since(began)
	slog.Info("video trial over", "file", path, "units", len(units), "shown", rep.Shown, "dropped", rep.Dropped,
		"took", took.Round(time.Millisecond), "at", rep.At, "orientation", rep.Orientation)
	return err
}
