package rtspd

import (
	"fmt"
	"log/slog"

	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/lib/onvif"
)

func served() []int { return livecam.Served() }

func helperProfiles() []onvif.Profile {
	sizes := livecam.Sizes()
	var out []onvif.Profile
	for _, i := range served() {
		sz := sizes[i]
		out = append(
			out,
			onvif.Profile{Token: paths[i], Name: streamName(i), Width: sz.Width, Height: sz.Height,
				FPS: livecam.FPS, Bitrate: livecam.Bitrate(sz.Width, sz.Height), Path: paths[i]},
		)
	}
	return out
}

func (p *pump) helper(stop chan struct{}) error {
	s, frames, err := livecam.Join(p.at)
	if err != nil {
		return err
	}
	defer livecam.Leave(s, frames)
	sz := livecam.Sizes()[p.at]
	slog.Info(
		"rtsp stream up",
		"stream",
		streamName(p.at),
		"size",
		fmt.Sprintf("%dx%d", sz.Width, sz.Height),
		"fps",
		livecam.FPS,
		"source",
		"lanovo-camera",
	)

	for {
		select {
		case <-stop:
			return nil
		case <-s.Done():
			return s.Err()
		case f := <-frames:
			if p.idle() || !p.allowed() {
				slog.Info("rtsp stream down", "stream", streamName(p.at))
				return nil
			}
			p.stream.Write(f.Data, f.PTS)
		}
	}
}
