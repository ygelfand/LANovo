package control

import (
	"context"
	"fmt"
	capture "github.com/ygelfand/libcountertop/pkg/audio/capture"

	"github.com/ygelfand/LANovo/internal/hardware/mic"
)

const echoChannels = 5

func recordEcho(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("record echo: how many seconds")
	}
	d, err := capture.Duration(args[0], shortest, longest)
	if err != nil {
		return "", err
	}

	path := ""
	if len(args) > 1 {
		path = args[1]
	}

	frames, err := capture.Events(ctx, d, func(receive func(mic.EchoFrame)) func() { return mic.Get().Echo.Listen(receive) }, func(f mic.EchoFrame) mic.EchoFrame {
		for i := range f.Mic {
			f.Mic[i] = append([]int16(nil), f.Mic[i]...)
			f.Out[i] = append([]int16(nil), f.Out[i]...)
		}
		f.Reference = append([]int16(nil), f.Reference...)
		return f
	})
	if err != nil {
		return "", err
	}
	return echoReport(frames, path)
}

type echoTrack struct {
	chans      [echoChannels][]int16
	cancelling int
	offset     int64
	aligned    bool
}

func flatten(frames []mic.EchoFrame) echoTrack {
	var t echoTrack
	for _, f := range frames {
		n := min(len(f.Mic[0]), len(f.Mic[1]), len(f.Out[0]), len(f.Out[1]))
		ref := f.Reference
		if len(ref) < n {
			ref = make([]int16, n)
		}
		for i, src := range [][]int16{f.Mic[0], f.Mic[1], ref, f.Out[0], f.Out[1]} {
			t.chans[i] = append(t.chans[i], src[:n]...)
		}
		if f.Cancelling {
			t.cancelling += n
		}
		t.offset, t.aligned = f.Offset, t.aligned || f.Aligned
	}
	return t
}

func echoReport(frames []mic.EchoFrame, path string) (string, error) {
	t := flatten(frames)
	n := len(t.chans[0])
	if n == 0 {
		return "", fmt.Errorf("record echo: nothing captured")
	}

	say := fmt.Sprintf("samples %d seconds %.2f rate %d aligned %v offset %d cancelling %.2fs",
		n, float64(n)/mic.Voice, mic.Voice, t.aligned, t.offset, float64(t.cancelling)/mic.Voice)
	for i, name := range []string{"micL", "micR", "ref", "outL", "outR"} {
		say += fmt.Sprintf(" | %s %.1f", name, levelDBFS(t.chans[i]))
	}

	if path == "" {
		return say, nil
	}
	samples := make([]int16, 0, n*echoChannels)
	for i := range n {
		for c := range echoChannels {
			samples = append(samples, t.chans[c][i])
		}
	}
	if err := capture.Write(path, samples, mic.Voice, echoChannels); err != nil {
		return "", err
	}
	return path + " " + say, nil
}

func levelDBFS(samples []int16) float64 { return capture.DBFS(capture.Measure(samples).RMS()) }
