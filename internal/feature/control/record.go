package control

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/lib/wave"
)

// The bounds on a capture. Long enough to say something into, short enough that a mistyped number
// does not hold the socket open for an hour.
const (
	shortest = time.Second
	longest  = 30 * time.Second
)

// record captures what the microphones hear and says how loud it was.
//
// The numbers are the point as much as the file is. Whether the mute slider actually cuts the
// microphone is answered by recording with it either way and comparing the peaks, which is a
// question about the hardware that no amount of reading the driver settles.
//
// It takes the levelled voice-rate stream, which is what detection and the pipeline are given, so
// what this captures is what they heard rather than a second opinion.
func record(args []string) (string, error) {
	if len(args) > 0 && args[0] == "raw" {
		return recordRaw(args[1:])
	}
	if len(args) > 0 && args[0] == "echo" {
		return recordEcho(args[1:])
	}
	if len(args) == 0 {
		return "", fmt.Errorf("record: how many seconds")
	}

	secs, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return "", fmt.Errorf("record: %s is not a number of seconds", args[0])
	}

	d := time.Duration(secs * float64(time.Second))
	if d < shortest || d > longest {
		return "", fmt.Errorf("record: %v is outside %v to %v", d, shortest, longest)
	}

	path := ""
	if len(args) > 1 {
		path = args[1]
	}

	frames, stop := mic.Get().Listen("ctl record")
	defer stop()

	pcm := make([]byte, 0, int(d.Seconds())*mic.Voice*2)
	var peak int
	var energy float64
	var count int

	over := time.After(d)
	for {
		select {
		case <-over:
			return wrote(path, pcm, peak, energy, count)
		case frame := <-frames:
			for _, s := range frame {
				pcm = append(pcm, byte(s), byte(s>>8))

				if v := int(s); v > peak {
					peak = v
				} else if -v > peak {
					peak = -v
				}
				energy += float64(s) * float64(s)
				count++
			}
		}
	}
}

// wrote saves the capture if it was asked for, and reports what was heard either way.
func wrote(path string, pcm []byte, peak int, energy float64, count int) (string, error) {
	var rms float64
	if count > 0 {
		rms = math.Sqrt(energy / float64(count))
	}

	// Full scale is what a level means: a peak of 300 says nothing until it is -41 dBFS, and the
	// difference between a live microphone and a cut one is tens of dB.
	say := fmt.Sprintf("samples %d seconds %.2f peak %d peakdbfs %.1f rms %.0f rmsdbfs %.1f",
		count, float64(count)/float64(mic.Voice),
		peak, dbfs(float64(peak)), math.Round(rms), dbfs(rms))

	if path == "" {
		return say, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("record: %w", err)
	}
	if err := os.WriteFile(path, wave.Mono16(pcm, mic.Voice), 0o644); err != nil {
		return "", fmt.Errorf("record: %w", err)
	}
	return path + " " + say, nil
}

// dbfs is a sample level against full scale. Silence is reported as the floor rather than as
// negative infinity, which formats badly and reads like an error.
func dbfs(v float64) float64 {
	if v < 1 {
		v = 1
	}
	return math.Round(20*math.Log10(v/32768)*10) / 10
}
