package visual

import (
	"image"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/ui"
)

type fakeGL struct {
	programs, textures int
	tex                image.Rectangle
	u                  []float32
	passes             Passes
	points             []Point
	segments           []Segment
	quads              []Quad
	amount             float32
}

func (f *fakeGL) Texture(unit int, img *image.RGBA) error {
	f.textures++
	f.tex = img.Bounds()
	return nil
}

func (f *fakeGL) Program(_ string, p Passes) error { f.programs++; f.passes = p; return nil }

func (f *fakeGL) Points(p []Point) error { f.points = p; return nil }

func (f *fakeGL) Lines(s []Segment) error { f.segments = s; return nil }

func (f *fakeGL) Quads(q []Quad) error { f.quads = q; return nil }

func (f *fakeGL) Values(u []float32, amount, _ float32, _ int) error {
	f.u, f.amount = u, amount
	return nil
}

func TestClassicVUShadesFromItsBackdrop(t *testing.T) {
	v := New(ClassicVU)
	g := &fakeGL{}
	in := ui.Rect{W: 960, H: 600}
	x := Input{Now: time.Second, Dt: time.Second / 60}
	if err := v.Shade(g, true, in, x); err != nil {
		t.Fatal(err)
	}
	if g.programs != 1 || g.textures != 1 || g.tex != image.Rect(0, 0, 960*shadeScale, 600*shadeScale) {
		t.Fatalf("programs %d, textures %d at %v", g.programs, g.textures, g.tex)
	}
	if len(g.u) != 24 || g.u[0] != shadeScale || g.u[1] <= 0 {
		t.Fatalf("values %v", g.u)
	}
	x.Now += time.Second / 60
	x.Speaker.Level = 0.5
	before := g.u[20]
	for range 10 {
		if err := v.Shade(g, false, in, x); err != nil {
			t.Fatal(err)
		}
		x.Now += time.Second / 60
	}
	if g.programs != 1 || g.textures != 1 {
		t.Errorf("sent again: programs %d, textures %d", g.programs, g.textures)
	}
	if g.u[20] <= before {
		t.Errorf("the speaker needle did not move: %v to %v", before, g.u[20])
	}
	if err := v.Shade(g, true, in, x); err != nil {
		t.Fatal(err)
	}
	if g.programs != 2 || g.textures != 2 {
		t.Errorf("a fresh layer was not refilled: programs %d, textures %d", g.programs, g.textures)
	}
}

func TestAuroraShadesWithItsGlowAndKeepsPhasesSmall(t *testing.T) {
	v := New(Aurora)
	g := &fakeGL{}
	in := ui.Rect{W: 960, H: 600}
	x := Input{Now: time.Second, Dt: time.Second / 60}
	if err := v.Shade(g, true, in, x); err != nil {
		t.Fatal(err)
	}
	if g.programs != 1 || g.textures != 3 || g.passes != Light|Pre || g.amount != 0.9 {
		t.Fatalf("programs %d, textures %d, passes %d, glow %v", g.programs, g.textures, g.passes, g.amount)
	}
	v.(*aurora).ph = 1e7
	x.Now = 90 * time.Hour
	if err := v.Shade(g, false, in, x); err != nil {
		t.Fatal(err)
	}
	if g.textures != 3 {
		t.Errorf("textures sent again: %d", g.textures)
	}
	for i := 10; i < 34; i++ {
		if g.u[i] < 0 || g.u[i] > 1024 {
			t.Errorf("u[%d] = %v, not wrapped", i, g.u[i])
		}
	}
	if g.u[2] < 0 || g.u[2] >= 3600 {
		t.Errorf("twinkle clock %v, not wrapped", g.u[2])
	}
}

func TestHaloShadesWithFeedbackAndSparks(t *testing.T) {
	v := New(Halo)
	g := &fakeGL{}
	in := ui.Rect{W: 960, H: 600}
	x := Input{Now: time.Second, Dt: time.Second / 30, Replying: true}
	x.Speaker.Level = 0.9
	for i := range x.Speaker.Bands {
		x.Speaker.Bands[i] = 0.8
	}
	if err := v.Shade(g, true, in, x); err != nil {
		t.Fatal(err)
	}
	if g.programs != 1 || g.passes != Light|Feed|FeedHalf {
		t.Fatalf("programs %d, passes %d", g.programs, g.passes)
	}
	for range 30 {
		x.Now += time.Second / 30
		if err := v.Shade(g, false, in, x); err != nil {
			t.Fatal(err)
		}
	}
	if len(g.points) == 0 || !g.points[0].Feed {
		t.Errorf("no sparks into the feedback: %d", len(g.points))
	}
	if g.u[0] <= 0.01 {
		t.Errorf("bars did not grow: %v", g.u[0])
	}
}

func TestGalaxyShadesItsStarsAsPoints(t *testing.T) {
	v := New(Galaxy)
	g := &fakeGL{}
	in := ui.Rect{W: 960, H: 600}
	x := Input{Now: time.Second, Dt: time.Second / 30}
	if err := v.Shade(g, true, in, x); err != nil {
		t.Fatal(err)
	}
	if g.programs != 1 || g.textures != 1 || g.passes != Light|Feed {
		t.Fatalf("programs %d, textures %d, passes %d", g.programs, g.textures, g.passes)
	}
	if len(g.points) != galaxyN || !g.points[0].Feed {
		t.Fatalf("%d points", len(g.points))
	}
	if err := v.Shade(g, false, in, x); err != nil {
		t.Fatal(err)
	}
	if g.textures != 1 {
		t.Errorf("backdrop sent again: %d", g.textures)
	}
}

func TestMotherShadesTextTraceAndLamps(t *testing.T) {
	v := New(Mother)
	g := &fakeGL{}
	in := ui.Rect{W: 960, H: 600}
	x := Input{Now: time.Second, Dt: time.Second / 30}
	if err := v.Shade(g, true, in, x); err != nil {
		t.Fatal(err)
	}
	if g.programs != 1 || g.textures != 4 || g.passes != Light {
		t.Fatalf("programs %d, textures %d, passes %d", g.programs, g.textures, g.passes)
	}
	if err := v.Shade(g, false, in, x); err != nil {
		t.Fatal(err)
	}
	if g.textures != 5 {
		t.Errorf("only the wave should be sent again: %d textures", g.textures)
	}
	if len(g.points) == 0 {
		t.Error("no lamps lit")
	}
}

func TestPaintSplatsItsBlobs(t *testing.T) {
	v := New(PaintSplash)
	g := &fakeGL{}
	in := ui.Rect{W: 960, H: 600}
	x := Input{Now: time.Second, Dt: time.Second / 30}
	if err := v.Shade(g, true, in, x); err != nil {
		t.Fatal(err)
	}
	if g.passes != Light|Feed|Splat {
		t.Fatalf("passes %d", g.passes)
	}
	splats := 0
	for _, p := range g.points {
		if p.Splat {
			splats++
		}
	}
	if splats == 0 {
		t.Error("the seed splash sent no blobs")
	}
}

func TestMercurySplatsItsBalls(t *testing.T) {
	v := New(Mercury)
	g := &fakeGL{}
	x := Input{Now: time.Second, Dt: time.Second / 30}
	if err := v.Shade(g, true, ui.Rect{W: 960, H: 600}, x); err != nil {
		t.Fatal(err)
	}
	if g.passes != Light|Splat || g.textures != 1 || len(g.points) != 7 {
		t.Fatalf("passes %d, textures %d, points %d", g.passes, g.textures, len(g.points))
	}
}

func TestSynthwaveShadesItsRidges(t *testing.T) {
	v := New(Synthwave)
	g := &fakeGL{}
	x := Input{Now: time.Second, Dt: time.Second / 30}
	if err := v.Shade(g, true, ui.Rect{W: 960, H: 600}, x); err != nil {
		t.Fatal(err)
	}
	if err := v.Shade(g, false, ui.Rect{W: 960, H: 600}, x); err != nil {
		t.Fatal(err)
	}
	if g.passes != Light || g.textures != 3 {
		t.Fatalf("passes %d, textures %d", g.passes, g.textures)
	}
}

func TestFireworksLaunchIntoTheFeedback(t *testing.T) {
	v := New(Fireworks)
	g := &fakeGL{}
	x := Input{Now: time.Second, Dt: time.Second / 30}
	for i := range 90 {
		x.Now = time.Second + time.Duration(i)*time.Second/30
		if err := v.Shade(g, i == 0, ui.Rect{W: 960, H: 600}, x); err != nil {
			t.Fatal(err)
		}
	}
	if g.passes != Light|Feed || g.textures != 2 || len(g.points) == 0 {
		t.Fatalf("passes %d, textures %d, points %d", g.passes, g.textures, len(g.points))
	}
}

func TestTeslaShadesFilamentsAsLines(t *testing.T) {
	v := New(Tesla)
	g := &fakeGL{}
	x := Input{Now: time.Second, Dt: time.Second / 30}
	if err := v.Shade(g, true, ui.Rect{W: 960, H: 600}, x); err != nil {
		t.Fatal(err)
	}
	if g.passes != Light|Lines || g.textures != 2 || len(g.segments) != 5*64*3 {
		t.Fatalf("passes %d, textures %d, segments %d", g.passes, g.textures, len(g.segments))
	}
}

func TestRibbonsShadeWithAColumnPass(t *testing.T) {
	v := New(Ribbons)
	g := &fakeGL{}
	x := Input{Now: time.Second, Dt: time.Second / 30}
	if err := v.Shade(g, true, ui.Rect{W: 960, H: 600}, x); err != nil {
		t.Fatal(err)
	}
	if g.passes != Light|Pre || len(g.u) != 58 || g.u[1] <= 0 {
		t.Fatalf("passes %d, values %v", g.passes, g.u)
	}
}

func TestOrbShadesPointsAndPlexus(t *testing.T) {
	v := New(Orb)
	g := &fakeGL{}
	x := Input{Now: time.Second, Dt: time.Second / 30}
	x.Mic.Level = 0.6
	if err := v.Shade(g, true, ui.Rect{W: 960, H: 600}, x); err != nil {
		t.Fatal(err)
	}
	if g.passes != Light|Lines || len(g.points) != orbN+orbRing {
		t.Fatalf("passes %d, points %d", g.passes, len(g.points))
	}
}

func TestDigitalVUShadesItsColumns(t *testing.T) {
	v := New(DigitalVU)
	g := &fakeGL{}
	if err := v.Shade(g, true, ui.Rect{W: 960, H: 600}, Input{Now: time.Second, Dt: time.Second / 30}); err != nil {
		t.Fatal(err)
	}
	if g.passes != Light || len(g.u) != 73 {
		t.Fatalf("passes %d, values %d", g.passes, len(g.u))
	}
}

func TestPhosphorShadesItsBeam(t *testing.T) {
	v := New(Phosphor)
	g := &fakeGL{}
	if err := v.Shade(g, true, ui.Rect{W: 960, H: 600}, Input{Now: time.Second, Dt: time.Second / 30}); err != nil {
		t.Fatal(err)
	}
	if g.passes != Light|Feed|Lines || g.textures != 2 || len(g.segments) == 0 {
		t.Fatalf("passes %d, textures %d, segments %d", g.passes, g.textures, len(g.segments))
	}
}

func TestPulsarUploadsItsRidges(t *testing.T) {
	v := New(Pulsar)
	g := &fakeGL{}
	if err := v.Shade(g, true, ui.Rect{W: 960, H: 600}, Input{Now: time.Second, Dt: time.Second / 30}); err != nil {
		t.Fatal(err)
	}
	if g.passes != Light || g.textures != 2 || g.tex != image.Rect(0, 0, pulsarP, pulsarN+2) {
		t.Fatalf("passes %d, textures %d, last %v", g.passes, g.textures, g.tex)
	}
}

func TestLCARSUploadsItsFrameOnce(t *testing.T) {
	v := New(LCARS)
	g := &fakeGL{}
	x := Input{Now: time.Second, Dt: time.Second / 30}
	if err := v.Shade(g, true, ui.Rect{W: 960, H: 600}, x); err != nil {
		t.Fatal(err)
	}
	first := g.textures
	x.Now += time.Second / 30
	if err := v.Shade(g, false, ui.Rect{W: 960, H: 600}, x); err != nil {
		t.Fatal(err)
	}
	if first != 3 || g.textures != 3 || g.passes != Light {
		t.Fatalf("textures %d then %d, passes %d", first, g.textures, g.passes)
	}
}

func TestHALUploadsItsPanelOnce(t *testing.T) {
	v := New(HAL9000)
	g := &fakeGL{}
	x := Input{Now: time.Second, Dt: time.Second / 30}
	if err := v.Shade(g, true, ui.Rect{W: 600, H: 960}, x); err != nil {
		t.Fatal(err)
	}
	x.Now += time.Second / 30
	if err := v.Shade(g, false, ui.Rect{W: 600, H: 960}, x); err != nil {
		t.Fatal(err)
	}
	if g.textures != 2 || g.passes != Light || len(g.u) != 14 {
		t.Fatalf("textures %d, passes %d, values %d", g.textures, g.passes, len(g.u))
	}
}

func TestLightCyclesShadeWallsAsQuads(t *testing.T) {
	v := New(LightCycles)
	g := &fakeGL{}
	x := Input{Now: time.Second, Dt: time.Second / 30}
	for range 30 {
		if err := v.Shade(g, g.programs == 0, ui.Rect{W: 600, H: 960}, x); err != nil {
			t.Fatal(err)
		}
		x.Now += time.Second / 30
	}
	if g.textures != 1 || g.passes != Light|Lines || len(g.quads) < 20 || len(g.segments) != 2*len(g.quads)+2 {
		t.Fatalf("textures %d, passes %d, quads %d, segments %d", g.textures, g.passes, len(g.quads), len(g.segments))
	}
}
