package lanovod

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/ygelfand/libcountertop/pkg/display/geometry"
	"github.com/ygelfand/libcountertop/pkg/display/video"
	"github.com/ygelfand/libcountertop/pkg/media/videostream"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

const videoFrame = time.Second / 30

type fileFrames struct {
	units [][]byte
	next  int
}

func (f *fileFrames) Next(ctx context.Context) (videostream.Frame, error) {
	if err := ctx.Err(); err != nil {
		return videostream.Frame{}, err
	}
	if f.next == len(f.units) {
		return videostream.Frame{}, io.EOF
	}
	fr := videostream.Frame{Data: f.units[f.next], At: time.Duration(f.next) * videoFrame}
	f.next++
	return fr, nil
}

func wallClock() videostream.Clock {
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
	codec, w, h, units := videostream.Split(data)
	if len(units) == 0 {
		return fmt.Errorf("no frames in %s", path)
	}
	began := time.Now()
	player := video.Player{
		Helper:         display.Get().Helper(),
		SecureDecoders: board.Current().SecureDecoders,
	}
	under := &videostream.Beneath{
		Rotation: func() geometry.Orientation { return display.Get().Orientation() },
		Size:     func() (int, int) { return display.Get().Native() },
	}
	rep, err := player.On(context.Background(), under, videostream.Stream{
		Codec: codec, Width: w, Height: h,
		Source: &fileFrames{units: units},
		Clock:  wallClock(),
	})
	took := time.Since(began)
	slog.Info(
		"video trial over",
		"file",
		path,
		"units",
		len(units),
		"shown",
		rep.Shown,
		"dropped",
		rep.Dropped,
		"took",
		took.Round(time.Millisecond),
		"at",
		rep.At,
		"orientation",
		rep.Orientation,
	)
	return err
}
