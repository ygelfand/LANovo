package gui

import (
	"context"
	"testing"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"
	backend "github.com/ygelfand/libcountertop/pkg/display/gogui"
)

type frames chan struct{}

func (frames) Program(int, string, string) error                          { return nil }
func (frames) Texture(uint32, int, int, int, int, int, int, []byte) error { return nil }
func (f frames) Frame(int, [4]float32, []float32, []uint32) (uint32, uint32, error) {
	select {
	case f <- struct{}{}:
	default:
	}
	return 0, 0, nil
}

func pressRig(t *testing.T, fired chan string) (*backend.Renderer, frames) {
	t.Helper()
	presentation.Reset()
	f := make(frames, 64)
	w := gogui.SimpleWindow("press", 400, 400, &struct{}{}, func(w *gogui.Window) {
		w.SetView(func(*gogui.Window) gogui.View {
			return gogui.Column(
				gogui.ContainerCfg{
					Sizing:     gogui.FillFill,
					Scrollable: true,
					OnGesture:  presentation.HoldStill,
					Content: []gogui.View{
						presentation.Presses.Pressable(
							gogui.Row,
							gogui.ContainerCfg{ID: "row", Sizing: gogui.FillFit, Height: 100},
							func(gogui.EventCtx) { fired <- "row" },
						),
						gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Height: 900}),
					},
				},
			)
		})
	})
	r, err := backend.New(f, w)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	t.Cleanup(func() { cancel(); <-done })
	go func() { defer close(done); _ = r.Run(ctx, w) }()
	<-f
	return r, f
}

func settle(f frames) {
	for {
		select {
		case <-f:
		case <-time.After(150 * time.Millisecond):
			return
		}
	}
}

func TestAPressFiresOnReleaseNotOnTouch(t *testing.T) {
	fired := make(chan string, 4)
	r, f := pressRig(t, fired)

	r.Touch(backend.Began, 1, 200, 50)
	settle(f)
	select {
	case got := <-fired:
		t.Fatalf("%s fired while the finger was still down", got)
	default:
	}

	r.Touch(backend.Ended, 1, 200, 50)
	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("nothing fired on release")
	}
	settle(f)
	if len(fired) != 0 {
		t.Error("fired twice for one tap")
	}
}

func TestADragThatStartsOnARowDoesNotFireIt(t *testing.T) {
	fired := make(chan string, 4)
	r, f := pressRig(t, fired)

	r.Touch(backend.Began, 1, 200, 80)
	for y := float32(75); y >= 20; y -= 5 {
		r.Touch(backend.Moved, 1, 200, y)
	}
	r.Touch(backend.Ended, 1, 200, 20)
	settle(f)
	if len(fired) != 0 {
		t.Errorf("%s fired after a drag", <-fired)
	}
}
