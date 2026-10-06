// Package livecam supplies board-specific configuration to the shared camera-session hub.
package livecam

import (
	"errors"
	"fmt"
	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/mtkcamera"
	camerasession "github.com/ygelfand/libcountertop/pkg/camera/session"
	"log/slog"
	"time"
)

const FPS = 30
const Wait = 2 * time.Second
const Settle = 2 * time.Second

var ErrNoStill = camerasession.ErrNoStill
var ErrMuted = camerasession.ErrMuted
var ErrRestarted = errors.New("livecam: the panel turned or the streams changed")

type Picture = camerasession.Picture
type Session = camerasession.Session
type Size struct{ Width, Height int }

// Sizes is each stream as it is encoded, turned to stand upright.
func Sizes() []Size { return sizesFor(Turn()) }

func sizesFor(q int) []Size {
	k := Saved()
	mw, mh := parseSize(k.MainSize)
	out := []Size{{mw, mh}}
	if k.SubOn {
		sw, sh := subFor(k)
		out = append(out, Size{sw, sh})
	}
	if q >= 0 && q%2 == 1 {
		for i := range out {
			out[i].Width, out[i].Height = out[i].Height, out[i].Width
		}
	}
	return out
}

// Turn is the quarter turns the helper draws the camera through.
func Turn() int {
	if board.Current().SoC == board.MediaTek {
		return 0
	}
	return turnFor(int(display.Get().Orientation()))
}

// mounted is the quarter turns clockwise that stand a Qualcomm board's frame up with the device at 0°.
const mounted = 3

func turnFor(device int) int {
	return (mounted - ((device/90)%4+4)%4 + 8) % 4
}

const Keyframe = 2

func Bitrate(w, h int) int { return bitrateFor(Saved(), w, h) }

var hub = camerasession.New(camerasession.Options{
	Snapshot: cameraSnapshot,
	Open:     func(c mtkcamera.Config) (camerasession.Transport, error) { return mtkcamera.Open(c) },
	Muted:    muted, Recover: revive, RestartError: ErrRestarted, Wait: Wait, Settle: Settle,
})

func cameraSnapshot() camerasession.Snapshot {
	q := Turn()
	k := Saved()
	sizes := sizesFor(q)
	cfg := mtkcamera.Config{Width: sizes[0].Width, Height: sizes[0].Height, FPS: FPS, Bitrate: bitrateFor(k, sizes[0].Width, sizes[0].Height), Keyframe: k.Keyframe, Params: Params(k), Turn: q, Mirror: board.Current().CameraMirror}
	if len(sizes) > 1 {
		cfg.SubWidth = sizes[1].Width
		cfg.SubHeight = sizes[1].Height
		cfg.SubBitrate = bitrateFor(k, sizes[1].Width, sizes[1].Height)
	}
	return camerasession.Snapshot{Config: cfg, Key: fmt.Sprintf("%d:%s", q, shapeOf(k))}
}
func Join(at int) (*Session, <-chan mtkcamera.Frame, error) { return hub.Join(at) }
func Leave(s *Session, frames <-chan mtkcamera.Frame)       { hub.Leave(s, frames) }
func RequestKey() error                                     { return hub.RequestKey() }
func Still(within time.Duration) (Picture, error)           { return hub.Still(within) }
func muted() bool                                           { return board.Current().MicMutesCamera && privacy.Get().MicMuted() }

const reviveEvery = 30 * time.Second

var revived time.Time

func revive(cause error) {
	if time.Since(revived) < reviveEvery {
		return
	}
	revived = time.Now()
	slog.Warn("restarting the camera helper", "err", cause)
	if err := prop.Restart(prop.Local, helperService); err != nil {
		slog.Error("restarting the camera helper failed", "err", err)
	}
}

const helperService = "lanovo_camera"
