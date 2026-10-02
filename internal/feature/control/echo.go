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

const echoChannels = 5

func recordEcho(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("record echo: how many seconds")
	}
	secs, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return "", fmt.Errorf("record echo: %s is not a number of seconds", args[0])
	}
	d := time.Duration(secs * float64(time.Second))
	if d < shortest || d > longest {
		return "", fmt.Errorf("record echo: %v is outside %v to %v", d, shortest, longest)
	}
	path := ""
	if len(args) > 1 {
		path = args[1]
	}

	var mu sync.Mutex
	var frames []mic.EchoFrame
	stop := mic.Get().Echo.Listen(func(f mic.EchoFrame) {
		mu.Lock()
		frames = append(frames, f)
		mu.Unlock()
	})
	time.Sleep(d)
	stop()

	mu.Lock()
	defer mu.Unlock()
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
	pcm := make([]byte, 0, n*echoChannels*2)
	for i := range n {
		for c := range echoChannels {
			s := t.chans[c][i]
			pcm = append(pcm, byte(s), byte(s>>8))
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("record echo: %w", err)
	}
	if err := os.WriteFile(path, wave.PCM16(pcm, mic.Voice, echoChannels), 0o644); err != nil {
		return "", fmt.Errorf("record echo: %w", err)
	}
	return path + " " + say, nil
}

func levelDBFS(s []int16) float64 {
	if len(s) == 0 {
		return dbfs(0)
	}
	var e float64
	for _, v := range s {
		e += float64(v) * float64(v)
	}
	return dbfs(math.Sqrt(e / float64(len(s))))
}
