package video

import (
	"github.com/ygelfand/libcountertop/pkg/media/playback"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

var heights = []int{360, 480, 720, 1080}

func Target() playback.Target {
	w, h := display.Get().Native()
	vw, vh := display.Get().Orientation().Size(w, h)
	return target(vw, vh, board.Current().MaxFPS)
}

func target(vw, vh, fastest int) playback.Target {
	drawn := min(vh, vw*9/16)
	tallest := heights[len(heights)-1]
	for _, t := range heights {
		if t >= drawn {
			tallest = t
			break
		}
	}
	return playback.Target{Width: vw, Height: vh, Tallest: tallest, Fastest: fastest}
}
