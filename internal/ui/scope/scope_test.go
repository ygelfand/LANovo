package scope

import (
	"math"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/display/theme"
	"github.com/ygelfand/libcountertop/pkg/display/ui"
)

var blank = theme.Color{R: 0x11, G: 0x11, B: 0x11}

func outside(img *ui.Image, in ui.Rect) int {
	w, h := img.Size()

	n := 0
	for y := range h {
		for x := range w {
			if in.Contains(x, y) || img.At(x, y) == blank {
				continue
			}
			n++
		}
	}
	return n
}

func painted(img *ui.Image, in ui.Rect) int {
	n := 0
	for y := in.Y; y < in.Y+in.H; y++ {
		for x := in.X; x < in.X+in.W; x++ {
			if img.At(x, y) != blank {
				n++
			}
		}
	}
	return n
}

func frames(columns int) map[string]Frame {
	out := map[string]Frame{
		"silence": Read(make([]int16, 960), columns),
		"quiet":   Read(tone(440, 48000, 960, 0.02), columns),
		"loud":    Read(tone(440, 48000, 960, 0.95), columns),
	}

	for what, f := range out {
		var k Keeper
		for range columns + 4 {
			f = k.Next(f, columns)
		}
		out[what] = f
	}
	return out
}

func TestNoScopePaintsOutsideItsBox(t *testing.T) {
	for _, box := range []ui.Rect{
		{X: 40, Y: 30, W: 200, H: 60},
		{X: 10, Y: 10, W: 40, H: 180},
		{X: 0, Y: 0, W: 300, H: 200},
		{X: 120, Y: 90, W: 8, H: 8},
	} {
		for _, k := range Kinds() {
			for what, f := range frames(box.W) {
				img := ui.NewImage(320, 240, blank)
				Of(k).Draw(img, box, f, theme.Color{R: 0xff, G: 0xff, B: 0xff}, theme.All[0])

				if n := outside(img, box); n != 0 {
					t.Errorf("%s drawing %s in %dx%d painted %d pixels outside its box",
						k, what, box.W, box.H, n)
				}
			}
		}
	}
}

func TestASilentWaveformIsALineNotAGap(t *testing.T) {
	box := ui.Rect{X: 0, Y: 0, W: 120, H: 40}

	img := ui.NewImage(120, 40, blank)
	Of(
		Wave,
	).Draw(img, box, Read(make([]int16, 960), box.W), theme.Color{R: 0xff, G: 0xff, B: 0xff}, theme.All[0])

	if got := painted(img, box); got < box.W {
		t.Errorf("a silent waveform painted %d pixels across %d columns, want at least one each",
			got, box.W)
	}
}

func TestAnEmptyBoxDrawsNothing(t *testing.T) {
	for _, k := range Kinds() {
		for _, box := range []ui.Rect{{}, {X: 10, Y: 10, W: 0, H: 20}, {X: 10, Y: 10, W: 20, H: 0}} {
			img := ui.NewImage(64, 64, blank)
			Of(k).Draw(img, box, Read(tone(440, 48000, 960, 0.9), 64),
				theme.Color{R: 0xff, G: 0xff, B: 0xff}, theme.All[0])

			if n := outside(img, ui.Rect{}); n != 0 {
				t.Errorf("%s painted %d pixels into a %dx%d box", k, n, box.W, box.H)
			}
		}
	}
}

func TestAnUnknownScopeFallsBack(t *testing.T) {
	if Of("does-not-exist") == nil {
		t.Fatal("an unknown scope is nil")
	}
	if Of("does-not-exist") != Of(Default) {
		t.Error("an unknown scope is not the default")
	}
}

func TestTheMeterRisesFastAndFallsSlowly(t *testing.T) {
	var m Meter

	loud := Frame{Peak: 1, RMS: 1}
	rose := m.Next(loud).RMS

	if rose < 0.4 {
		t.Errorf("one loud frame moved the meter to %.2f, want it to react at once", rose)
	}

	quiet := Frame{}
	fell := m.Next(quiet).RMS

	if fell >= rose {
		t.Errorf("the meter read %.2f then %.2f, want it falling", rose, fell)
	}
	if fell < rose*0.5 {
		t.Errorf("the meter fell from %.2f to %.2f in one frame, want a slow decay", rose, fell)
	}
}

func TestThePeakHangsThenFalls(t *testing.T) {
	var m Meter
	m.Next(Frame{Peak: 1, RMS: 1})

	held := m.Next(Frame{}).Hold
	if held < 0.99 {
		t.Fatalf("the peak dropped to %.2f immediately, want it held", held)
	}

	for range hangFor {
		m.Next(Frame{})
	}
	if after := m.Next(Frame{}).Hold; after >= held {
		t.Errorf("the peak still reads %.2f after the hang, want it falling", after)
	}
}

func TestSilenceReducesToTheSameFrameEveryTime(t *testing.T) {
	a := Read(make([]int16, 960), 64)
	b := Read(make([]int16, 960), 64)

	if a.Peak != b.Peak || a.RMS != b.RMS {
		t.Errorf("two silent periods read %v and %v", a, b)
	}
	for i := range a.Columns {
		if a.Columns[i] != b.Columns[i] {
			t.Fatalf("column %d differs between two silent periods", i)
		}
	}
	for i := range a.Bands {
		if a.Bands[i] != b.Bands[i] {
			t.Fatalf("band %d differs between two silent periods", i)
		}
	}
	if !a.Quiet() {
		t.Error("a silent period does not read as quiet")
	}
}

func TestReadMeasuresTheLevel(t *testing.T) {
	f := Read(tone(440, 48000, 960, 0.5), 32)

	if math.Abs(f.Peak-0.5) > 0.02 {
		t.Errorf("peak %.3f for a half scale tone, want about 0.5", f.Peak)
	}

	// A sine's RMS is its peak over root two.
	if want := 0.5 / math.Sqrt2; math.Abs(f.RMS-want) > 0.02 {
		t.Errorf("rms %.3f for a half scale sine, want about %.3f", f.RMS, want)
	}
}

func marked(at float64) Frame {
	bands := make([]float64, Bands)
	for i := range bands {
		bands[i] = at
	}
	return Frame{Bands: bands}
}

func TestTheTrailKeepsTheNewestColumnsInOrder(t *testing.T) {
	const width = 4

	var k Keeper
	var f Frame
	for i := range 10 {
		f = k.Next(marked(float64(i)/10), width)
	}

	if len(f.Trail) != width {
		t.Fatalf("the trail is %d columns after ten frames, want %d", len(f.Trail), width)
	}
	for i, want := range []float64{0.6, 0.7, 0.8, 0.9} {
		if got := f.Trail[i][0]; math.Abs(got-want) > 0.001 {
			t.Errorf("column %d is %.1f, want %.1f", i, got, want)
		}
	}
}

func TestTheTrailGrowsBeforeItIsFull(t *testing.T) {
	var k Keeper

	f := k.Next(marked(0.5), 8)
	if len(f.Trail) != 1 {
		t.Errorf("one frame gave %d columns, want 1", len(f.Trail))
	}

	f = k.Next(marked(0.5), 8)
	if len(f.Trail) != 2 {
		t.Errorf("two frames gave %d columns, want 2", len(f.Trail))
	}
}

func TestAResizedTrailStartsAgain(t *testing.T) {
	var k Keeper
	for range 6 {
		k.Next(marked(0.5), 6)
	}

	if f := k.Next(marked(0.5), 12); len(f.Trail) != 1 {
		t.Errorf("a trail resized to 12 has %d columns, want it starting again at 1", len(f.Trail))
	}
}

func TestTheSpectrogramWithoutATrailDrawsItsBox(t *testing.T) {
	box := ui.Rect{X: 4, Y: 4, W: 40, H: 20}

	img := ui.NewImage(64, 32, blank)
	Of(Spectrogram).Draw(img, box, Read(tone(440, 48000, 960, 0.9), box.W),
		theme.Color{R: 0xff, G: 0xff, B: 0xff}, theme.All[0])

	if n := outside(img, box); n != 0 {
		t.Errorf("a spectrogram with no trail painted %d pixels outside its box", n)
	}
}

func TestTheNewestColumnIsOnTheRight(t *testing.T) {
	box := ui.Rect{X: 0, Y: 0, W: 16, H: 16}
	palette := theme.All[0]

	var k Keeper
	for range box.W - 1 {
		k.Next(marked(0), box.W)
	}
	f := k.Next(marked(1), box.W)

	img := ui.NewImage(box.W, box.H, blank)
	Of(Spectrogram).Draw(img, box, f, theme.Color{R: 0xff, G: 0xff, B: 0xff}, palette)

	if got := img.At(box.W-1, box.H/2); got == palette.Surface {
		t.Errorf("the rightmost column is the surface color, want the newest frame drawn there")
	}
	if got := img.At(0, box.H/2); got != palette.Surface {
		t.Errorf("the leftmost column is %v, want the surface for a silent column", got)
	}
}
