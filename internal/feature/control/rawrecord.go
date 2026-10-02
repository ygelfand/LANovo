package control

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/lib/wave"
)

type channelStats struct {
	peak   int
	energy float64
}

func recordRaw(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("record raw: how many seconds")
	}
	secs, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return "", fmt.Errorf("record raw: %s is not a number of seconds", args[0])
	}
	d := time.Duration(secs * float64(time.Second))
	if d < shortest || d > longest {
		return "", fmt.Errorf("record raw: %v is outside %v to %v", d, shortest, longest)
	}
	path := ""
	if len(args) > 1 {
		path = args[1]
	}

	var mu sync.Mutex
	var samples []int16
	stop := mic.Get().Frames.Listen(func(f mic.Frame) {
		mu.Lock()
		samples = append(samples, f.Samples...)
		mu.Unlock()
	})
	time.Sleep(d)
	stop()

	mu.Lock()
	defer mu.Unlock()
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
		say += fmt.Sprintf(" | mic%d peakdbfs %.1f rmsdbfs %.1f", c+1, dbfs(float64(ch[c].peak)), dbfs(rms))
	}
	if ch[0].energy > 0 && ch[1].energy > 0 {
		say += fmt.Sprintf(" | correlation %.2f", cross/math.Sqrt(ch[0].energy*ch[1].energy))
	}

	if path == "" {
		return say, nil
	}
	pcm := make([]byte, 0, len(samples)*2)
	for _, s := range samples[:frames*mic.Channels] {
		pcm = append(pcm, byte(s), byte(s>>8))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("record raw: %w", err)
	}
	if err := os.WriteFile(path, wave.PCM16(pcm, mic.Rate, mic.Channels), 0o644); err != nil {
		return "", fmt.Errorf("record raw: %w", err)
	}
	return path + " " + say, nil
}
