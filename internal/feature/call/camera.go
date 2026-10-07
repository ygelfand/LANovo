package call

import (
	"context"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/hardware/mtkcamera"
	"github.com/ygelfand/LANovo/internal/lib/rtc"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
	"log/slog"
	"time"
)

const rejoinWait = 500 * time.Millisecond

func cameraStream() (int, livecam.Size, bool) {
	sizes := livecam.Sizes()
	if len(sizes) == 0 {
		return 0, livecam.Size{}, false
	}
	if config.Get().Call.Stream == config.CallSub && len(sizes) > 1 {
		return 1, sizes[1], true
	}
	return 0, sizes[0], true
}

type camera struct {
	at    int
	sized func(livecam.Size)
}

func (camera) Key() {
	if err := livecam.RequestKey(); err != nil {
		slog.Debug("call keyframe not requested", "err", err)
	}
}

func (c camera) Frames() (<-chan rtc.Frame, func()) {
	out := make(chan rtc.Frame, 8)
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	safe.Go("call camera", func() {
		defer close(done)
		defer close(out)
		var size livecam.Size
		for ctx.Err() == nil {
			s, frames, err := livecam.Join(c.at)
			if err != nil {
				slog.Warn("call camera", "err", err)
			} else {
				if sizes := livecam.Sizes(); c.at < len(sizes) && sizes[c.at] != size {
					size = sizes[c.at]
					if c.sized != nil {
						c.sized(size)
					}
				}
				pump(ctx, s, frames, out)
				livecam.Leave(s, frames)
			}
			select {
			case <-ctx.Done():
			case <-time.After(rejoinWait):
			}
		}
	})
	return out, func() { stop(); <-done }
}

func pump(ctx context.Context, s *livecam.Session, frames <-chan mtkcamera.Frame, out chan<- rtc.Frame) {
	var config []byte
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.Done():
			return
		case f, ok := <-frames:
			if !ok {
				return
			}
			if f.Config {
				config = append([]byte(nil), f.Data...)
				continue
			}
			data := f.Data
			if f.Key && config != nil {
				data = append(append([]byte(nil), config...), f.Data...)
			}
			select {
			case out <- rtc.Frame{Data: data, PTS: f.PTS, Key: f.Key}:
			default:
			}
		}
	}
}
