package gogui

import (
	"context"

	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

type frame struct {
	quads []float32
	runs  []uint32
}

type sink struct {
	programs map[int]bool
	frames   []frame
}

func (s *sink) Program(slot int, vs, fs string) error {
	if s.programs == nil {
		s.programs = map[int]bool{}
	}
	s.programs[slot] = vs != "" && fs != ""
	return nil
}

func (s *sink) Texture(uint32, int, int, int, int, int, int, []byte) error { return nil }

func (s *sink) Frame(_ int, _ [4]float32, quads []float32, runs []uint32) (uint32, uint32, error) {
	s.frames = append(s.frames, frame{append([]float32(nil), quads...), append([]uint32(nil), runs...)})
	return 0, 0, nil
}

type app struct{ on bool }

func view(w *gui.Window) gui.View {
	a := gui.State[app](w)
	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		Content: []gui.View{
			gui.ProgressBar(gui.ProgressBarCfg{Percent: 0.4, Width: 200}),
			gui.Switch(gui.SwitchCfg{Selected: a.on}),
		},
	})
}

func TestAFrameIsOneWellFormedBatch(t *testing.T) {
	s := &sink{}
	w := gui.SimpleWindow("test", 320, 200, &app{on: true}, func(w *gui.Window) { w.SetView(view) })
	r, err := New(s, w)
	if err != nil {
		t.Fatal(err)
	}
	if !s.programs[slotSolid] || !s.programs[slotGlyph] {
		t.Fatalf("programs %v", s.programs)
	}
	w.Config.OnInit(w)

	st, err := r.Render(w)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.frames) != 1 || st.Quads == 0 {
		t.Fatalf("%d frames, %d quads", len(s.frames), st.Quads)
	}
	f := s.frames[0]
	if len(f.quads)%quadFloats != 0 || len(f.runs)%runWords != 0 {
		t.Fatalf("ragged batch: %d floats, %d run words", len(f.quads), len(f.runs))
	}
	n := uint32(len(f.quads) / quadFloats)
	var covered uint32
	for i := 0; i < len(f.runs); i += runWords {
		first, count := f.runs[i+6], f.runs[i+7]
		if count == 0 || first != covered || first+count > n {
			t.Fatalf("run %d covers %d+%d of %d quads (expected to start at %d)", i/runWords, first, count, n, covered)
		}
		covered += count
	}
	if covered != n {
		t.Errorf("runs cover %d of %d quads", covered, n)
	}
}

func TestPackedParamsUnpackToRadiusAndThickness(t *testing.T) {
	p := packParams(12.25, 1.5)
	radius := float32(int(p/4096)) / 4
	thickness := float32(int(p)%4096) / 4
	if radius != 12.25 || thickness != 1.5 {
		t.Errorf("unpacked %v, %v", radius, thickness)
	}
}

type counter struct{ taps int }

type countingSink struct {
	sink
	frames chan int
}

func (s *countingSink) Frame(rot int, clear [4]float32, quads []float32, runs []uint32) (uint32, uint32, error) {
	s.frames <- len(quads) / quadFloats
	return 0, 0, nil
}

func TestATouchReachesTheViewAndDrawsAFrame(t *testing.T) {
	s := &countingSink{frames: make(chan int, 16)}
	taps := make(chan int, 4)
	w := gui.SimpleWindow("loop", 320, 200, &counter{}, func(w *gui.Window) {
		w.SetView(func(w *gui.Window) gui.View {
			return gui.Button(gui.ButtonCfg{
				ID:     "all",
				Label:  "",
				Sizing: gui.FillFill,
				OnClick: func(e gui.EventCtx) {
					c := gui.State[counter](e.Window)
					c.taps++
					taps <- c.taps
				},
			})
		})
	})
	r, err := New(s, w)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx, w) }()

	select {
	case <-s.frames:
	case <-time.After(2 * time.Second):
		t.Fatal("no first frame")
	}
	r.Touch(Began, 1, 160, 100)
	r.Touch(Ended, 1, 160, 100)
	select {
	case n := <-taps:
		if n != 1 {
			t.Errorf("%d taps", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the tap never reached the button")
	}
	cancel()
	r.poke()
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop")
	}
}

type textureSink struct {
	sink
	uploads []uint32
}

func (s *textureSink) Texture(id uint32, _, _, _, _, _, _ int, _ []byte) error {
	s.uploads = append(s.uploads, id)
	return nil
}

func TestAMemoryImageUploadsOnceAndItsSlotIsReusedWhenItLeaves(t *testing.T) {
	pix := make([]byte, 8*8*4)
	first := gui.UseImage("gogui-test/a", 8, 8, pix)
	second := gui.UseImage("gogui-test/b", 8, 8, pix)
	src := first
	w := gui.SimpleWindow("mem", 64, 64, &counter{}, func(w *gui.Window) {
		w.SetView(func(*gui.Window) gui.View {
			return gui.Image(gui.ImageCfg{Src: src, Width: 8, Height: 8})
		})
	})
	s := &textureSink{}
	r, err := New(s, w)
	if err != nil {
		t.Fatal(err)
	}
	w.Config.OnInit(w)
	base := len(s.uploads)
	for range 3 {
		if _, err := r.Render(w); err != nil {
			t.Fatal(err)
		}
	}
	got := s.uploads[base:]
	if len(got) != 1 {
		t.Fatalf("%d uploads for one unchanged image", len(got))
	}
	src = second
	if _, err := r.Render(w); err != nil {
		t.Fatal(err)
	}
	src = first
	if _, err := r.Render(w); err != nil {
		t.Fatal(err)
	}
	got = s.uploads[base:]
	if len(got) != 3 || got[2] != got[0] && got[2] != got[1] {
		t.Errorf("uploads %v: a returning image should reuse a freed slot", got)
	}
}

func TestAStillViewStopsDrawing(t *testing.T) {
	s := &countingSink{frames: make(chan int, 1024)}
	w := gui.SimpleWindow("still", 320, 200, &counter{}, func(w *gui.Window) {
		w.SetView(func(*gui.Window) gui.View {
			return gui.Column(gui.ContainerCfg{Sizing: gui.FillFill, Content: []gui.View{gui.Text(gui.TextCfg{Text: "12:00"})}})
		})
	})
	r, err := New(s, w)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.Run(ctx, w)
	time.Sleep(500 * time.Millisecond)
	if n := len(s.frames); n > 3 {
		t.Errorf("%d frames in half a second of a still view", n)
	}
}
