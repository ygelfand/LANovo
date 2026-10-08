package control

import (
	"context"
	"fmt"
	"math"

	capture "github.com/ygelfand/libcountertop/pkg/audio/capture"

	"github.com/ygelfand/LANovo/internal/hardware/mic"
)

type channelStats struct {
	peak   int
	energy float64
}

func recordRaw(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("record raw: how many seconds")
	}
	d, err := capture.Duration(args[0], shortest, longest)
	if err != nil {
		return "", err
	}

	path := ""
	if len(args) > 1 {
		path = args[1]
	}

	frames, err := capture.Events(
		ctx,
		d,
		func(receive func(mic.Frame)) func() { return mic.Get().Frames.Listen(receive) },
		func(f mic.Frame) mic.Frame { f.Samples = append([]int16(nil), f.Samples...); return f },
	)
	if err != nil {
		return "", err
	}
	var samples []int16
	for _, f := range frames {
		samples = append(samples, f.Samples...)
	}
	return rawReport(samples, path)
}

func rawReport(samples []int16, path string) (string, error) {
	var ch [mic.Channels]channelStats
	var cross float64
	frames := len(samples) / mic.Channels
	for i := range frames {
		for c := range mic.Channels {
			v := int(samples[i*mic.Channels+c])
			ch[c].peak = max(ch[c].peak, v, -v)
			ch[c].energy += float64(v) * float64(v)
		}
		cross += float64(samples[i*mic.Channels]) * float64(samples[i*mic.Channels+1])
	}
	if frames == 0 {
		return "", fmt.Errorf("record raw: nothing captured")
	}
	say := fmt.Sprintf("frames %d seconds %.2f rate %d", frames, float64(frames)/mic.Rate, mic.Rate)
	for c := range mic.Channels {
		rms := math.Sqrt(ch[c].energy / float64(frames))
		say += fmt.Sprintf(
			" | mic%d peakdbfs %.1f rmsdbfs %.1f",
			c+1,
			dbfs(float64(ch[c].peak)),
			dbfs(rms),
		)
	}
	if ch[0].energy > 0 && ch[1].energy > 0 {
		say += fmt.Sprintf(" | correlation %.2f", cross/math.Sqrt(ch[0].energy*ch[1].energy))
	}

	if path == "" {
		return say, nil
	}
	if err := capture.Write(
		path,
		samples[:frames*mic.Channels],
		mic.Rate,
		mic.Channels,
	); err != nil {
		return "", err
	}
	return path + " " + say, nil
}
