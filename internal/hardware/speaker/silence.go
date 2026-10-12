package speaker

import (
	"io"

	"github.com/ygelfand/libcountertop/pkg/audio/alsa"
)

func silence(out *alsa.Playback) (func() error, error) {
	if out == nil {
		return func() error { return nil }, nil
	}
	return silenceWriter(out.Write)
}

func silenceWriter(write func([]byte) (int, error)) (func() error, error) {
	quiet := make([]byte, period*FrameBytes)
	// ALSA starts playback on the first successful write. Finish it before
	// returning so amplifier programming cannot race the I2S clock startup.
	n, err := write(quiet)
	if err != nil {
		return nil, err
	}
	if n <= 0 {
		return nil, io.ErrNoProgress
	}

	quit, done := make(chan struct{}), make(chan error, 1)
	go func() {
		for {
			select {
			case <-quit:
				done <- nil
				return
			default:
			}
			if _, err := write(quiet); err != nil {
				done <- err
				return
			}
		}
	}()
	return func() error {
		close(quit)
		return <-done
	}, nil
}
