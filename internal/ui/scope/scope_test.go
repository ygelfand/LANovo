package scope

import (
	"math"
	"testing"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

var blank = theme.Color{R: 0x11, G: 0x11, B: 0x11}

// outside counts painted pixels beyond a box. A scope goes wherever a host puts it, often over
// something else, so painting outside overwrites whatever that was — the same contract the clock
// faces are held to by their sweep test.
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

// frames worth drawing: silence, something quiet, and something loud. Each carries a trail, so the
// spectrogram is held to the same contract with something in it rather than empty.
func frames(columns int) map[string]Frame {
	out := map[string]Frame{
		"silence": Read(make([]int16, 960), columns),
		"quiet":   Read(tone(440, 48000, 960, 0.02), columns),
		"loud":    Read(tone(440, 48000, 960, 0.95), columns),
	}

	for what, f := range out {
		var k Keeper
		// More frames than the box is wide, so the trail is full and has been shifted.
		for range columns + 4 {
			f = k.Next(f, columns)
		}
		out[what] = f
	}
	return out
}

// Every scope, every shape of box, every kind of sound: nothing outside the box.
func TestNoScopePaintsOutsideItsBox(t *testing.T) {
	for _, box := range []ui.Rect{
		{X: 40, Y: 30, W: 200, H: 60}, // a strip, as along the foot of a clock
		{X: 10, Y: 10, W: 40, H: 180}, // tall, as down the side of something
		{X: 0, Y: 0, W: 300, H: 200},  // the corner, where an off by one shows
		{X: 120, Y: 90, W: 8, H: 8},   // barely there
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

// Silence still draws something for the waveform — a line through the middle rather than a gap,
// because a waveform with holes in it reads as the drawing having failed.
func TestASilentWaveformIsALineNotAGap(t *testing.T) {
	box := ui.Rect{X: 0, Y: 0, W: 120, H: 40}

	img := ui.NewImage(120, 40, blank)
	Of(Wave).Draw(img, box, Read(make([]int16, 960), box.W), theme.Color{R: 0xff, G: 0xff, B: 0xff}, theme.All[0])

	if got := painted(img, box); got < box.W {
		t.Errorf("a silent waveform painted %d pixels across %d columns, want at least one each",
			got, box.W)
	}
}

// A zero box is a host with nothing to spare, not a reason to panic or to paint.
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

// Of falls back rather than handing back nothing: a name this build does not have should draw the
// default, not crash the screen that asked for it.
func TestAnUnknownScopeFallsBack(t *testing.T) {
	if Of("does-not-exist") == nil {
		t.Fatal("an unknown scope is nil")
	}
	if Of("does-not-exist") != Of(Default) {
		t.Error("an unknown scope is not the default")
	}
}

// The meter is what makes a level readable: it rises fast, falls slowly, and the peak hangs before
// it drops. Without the hold it is a bar that flickers.
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

// A frame is compared to decide whether to redraw, so silence has to reduce to the same frame every
// time: fifty identical periods a second must not each cost a repaint.
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

// marked is a frame whose spectrum is a single value, so a column can be told apart from its
// neighbours once it is in a trail.
func marked(at float64) Frame {
	bands := make([]float64, Bands)
	for i := range bands {
		bands[i] = at
	}
	return Frame{Bands: bands}
}

// The trail keeps the last width columns, newest last. Anything else and the spectrogram draws
// time backwards or grows without bound.
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

// A resized box starts again rather than showing a trail cut to a width it was never kept at.
func TestAResizedTrailStartsAgain(t *testing.T) {
	var k Keeper
	for range 6 {
		k.Next(marked(0.5), 6)
	}

	if f := k.Next(marked(0.5), 12); len(f.Trail) != 1 {
		t.Errorf("a trail resized to 12 has %d columns, want it starting again at 1", len(f.Trail))
	}
}

// A trail with nothing in it is a host that has not fed one yet, not a reason to panic.
func TestTheSpectrogramWithoutATrailDrawsItsBox(t *testing.T) {
	box := ui.Rect{X: 4, Y: 4, W: 40, H: 20}

	img := ui.NewImage(64, 32, blank)
	Of(Spectrogram).Draw(img, box, Read(tone(440, 48000, 960, 0.9), box.W),
		theme.Color{R: 0xff, G: 0xff, B: 0xff}, theme.All[0])

	if n := outside(img, box); n != 0 {
		t.Errorf("a spectrogram with no trail painted %d pixels outside its box", n)
	}
}

// Time runs left to right, so the newest column is the rightmost one. Drawn against a trail whose
// only loud column is the newest.
func TestTheNewestColumnIsOnTheRight(t *testing.T) {
	box := ui.Rect{X: 0, Y: 0, W: 16, H: 16}
	palette := theme.All[0]

	var k Keeper
	var f Frame
	for range box.W - 1 {
		f = k.Next(marked(0), box.W)
	}
	f = k.Next(marked(1), box.W)

	img := ui.NewImage(box.W, box.H, blank)
	Of(Spectrogram).Draw(img, box, f, theme.Color{R: 0xff, G: 0xff, B: 0xff}, palette)

	// Every quiet column is left as the surface; the loud one is not.
	if got := img.At(box.W-1, box.H/2); got == palette.Surface {
		t.Errorf("the rightmost column is the surface color, want the newest frame drawn there")
	}
	if got := img.At(0, box.H/2); got != palette.Surface {
		t.Errorf("the leftmost column is %v, want the surface for a silent column", got)
	}
}
