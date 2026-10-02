package control

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/display"
)

// benchFrames is how many frames a measurement takes when nobody says.
//
// Enough that one slow frame does not carry the average, few enough that the answer comes back
// while somebody is waiting for it: at the measured cost this is under a second.
const benchFrames = 60

// benchLimit is the longest a measurement may hold the render loop.
//
// It holds the panel for as long as it runs, so nothing else gets drawn meanwhile — a volume
// notice or an alarm would wait. Asking for a hundred thousand frames should be refused rather
// than obeyed.
const benchLimit = 5 * time.Second

// fps drives the real pipeline in a loop and reports what a frame costs.
//
// The question it answers is the one #8 could not: the UI is event driven and idles at nothing, so
// there is no animation to time and the frame counters only say what the device happened to draw.
// This asks instead what the pipeline would sustain.
func fps(args []string) (string, error) {
	frames := benchFrames
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			return "", fmt.Errorf("frames: %w", err)
		}
		if n < 1 {
			return "", fmt.Errorf("ask for at least one frame")
		}
		frames = n
	}

	ctx, cancel := context.WithTimeout(context.Background(), benchLimit)
	defer cancel()

	b, err := display.Get().Bench(ctx, frames)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}
