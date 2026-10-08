package control

import (
	"context"
	"fmt"
	"math"
	"time"

	capture "github.com/ygelfand/libcountertop/pkg/audio/capture"

	"github.com/ygelfand/LANovo/internal/hardware/mic"
)

const (
	shortest = time.Second
	longest  = 30 * time.Second
)

func record(ctx context.Context, args []string) (string, error) {
	if len(args) > 0 && args[0] == "raw" {
		return recordRaw(ctx, args[1:])
	}
	if len(args) > 0 && args[0] == "echo" {
		return recordEcho(ctx, args[1:])
	}
	if len(args) == 0 {
		return "", fmt.Errorf("record: how many seconds")
	}

	d, err := capture.Duration(args[0], shortest, longest)
	if err != nil {
		return "", err
	}

	path := ""
	if len(args) > 1 {
		path = args[1]
	}

	frames, stop := mic.Get().Listen("ctl audio record")
	defer stop()

	pcm := make([]byte, 0, int(d.Seconds())*mic.Voice*2)
	var peak int
	var energy float64
	var count int

	err = capture.Listen(ctx, d, frames, func(frame []int16) {
		for _, v := range frame {
			pcm = append(pcm, byte(v), byte(v>>8))
			peak = max(peak, int(v), -int(v))
			energy += float64(v) * float64(v)
			count++
		}
	})
	if err != nil {
		return "", err
	}
	if count == 0 {
		return "", fmt.Errorf("record: nothing captured")
	}
	return wrote(path, pcm, peak, energy, count)
}

func wrote(path string, pcm []byte, peak int, energy float64, count int) (string, error) {
	var rms float64
	if count > 0 {
		rms = math.Sqrt(energy / float64(count))
	}

	say := fmt.Sprintf("samples %d seconds %.2f peak %d peakdbfs %.1f rms %.0f rmsdbfs %.1f",
		count, float64(count)/float64(mic.Voice),
		peak, dbfs(float64(peak)), math.Round(rms), dbfs(rms))

	if path == "" {
		return say, nil
	}

	samples := make([]int16, len(pcm)/2)
	for i := range samples {
		samples[i] = int16(uint16(pcm[2*i]) | uint16(pcm[2*i+1])<<8)
	}
	if err := capture.Write(path, samples, mic.Voice, 1); err != nil {
		return "", err
	}
	return path + " " + say, nil
}

var dbfs = capture.DBFS
