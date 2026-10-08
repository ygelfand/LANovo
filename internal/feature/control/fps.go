package control

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/display"
)

const benchFrames = 60

const benchLimit = 5 * time.Second

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
