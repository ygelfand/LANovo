package visual

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/analysis"
	"github.com/ygelfand/LANovo/internal/ui"
)

var shapes = []struct {
	name   string
	screen ui.Rect
	box    ui.Rect
}{
	{"portrait-full", ui.Rect{W: 1200, H: 1920}, ui.Rect{W: 1200, H: 1920}},
	{"landscape-full", ui.Rect{W: 1920, H: 1200}, ui.Rect{W: 1920, H: 1200}},
	{"portrait-compact", ui.Rect{W: 1200, H: 1920}, ui.Rect{X: 40, Y: 1340, W: 1120, H: 540}},
	{"landscape-compact", ui.Rect{W: 1920, H: 1200}, ui.Rect{X: 40, Y: 760, W: 1840, H: 400}},
}

func loud() analysis.Analysis {
	var a analysis.Analysis
	a.Level, a.Peak, a.Onsets = 0.4, 0.7, 3
	for b := range a.Bands {
		a.Bands[b] = float32(0.35 + 0.3*math.Sin(float64(b)*0.55) + 0.25*math.Cos(float64(b)*0.21))
	}
	for i := range a.Wave {
		p := 2 * math.Pi * float64(i) / float64(len(a.Wave))
		a.Wave[i] = float32(0.55*math.Sin(3*p) + 0.3*math.Sin(7*p+1) + 0.1*math.Sin(13*p))
	}
	return a
}

func shades(t *testing.T, v Visual, box ui.Rect, n int) *fakeGL {
	t.Helper()
	g := &fakeGL{}
	x := Input{Mic: loud(), Speaker: loud(), Dt: 40 * time.Millisecond, Label: "LANOVO"}
	for i := range n {
		x.Now = time.Duration(i+1) * x.Dt
		x.Replying = i >= n/2
		if err := v.Shade(g, i == 0, box, x); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

func TestEveryVisualShadesEveryShape(t *testing.T) {
	for _, k := range Built() {
		for _, sh := range shapes {
			g := shades(t, New(k), sh.box, 3)
			if g.programs != 1 || len(g.u) == 0 {
				t.Errorf("%s %s: %d programs, %d values", k, sh.name, g.programs, len(g.u))
			}
		}
	}
}

func TestATinyBoxIsSafe(t *testing.T) {
	for _, k := range Built() {
		for _, box := range []ui.Rect{{}, {W: 1, H: 1}, {X: 5, Y: 5, W: 3, H: 40}} {
			shades(t, New(k), box, 2)
		}
	}
}

func TestEveryKindHasALabel(t *testing.T) {
	if len(Kinds()) != 33 {
		t.Errorf("%d kinds, want 33", len(Kinds()))
	}
	for _, k := range Kinds() {
		if k.Label() == "visual."+string(k) {
			t.Errorf("%s has no label", k)
		}
	}
}

func TestTheBuiltVisualsAreAlphabetical(t *testing.T) {
	b := Built()
	for i := 1; i < len(b); i++ {
		if strings.ToLower(b[i-1].Label()) > strings.ToLower(b[i].Label()) {
			t.Errorf("%q comes before %q", b[i-1].Label(), b[i].Label())
		}
	}
}

func TestAPinnedSeedRepeats(t *testing.T) {
	SetSeed(7)
	defer SetSeed(0)
	a, b := seeded(3), seeded(3)
	for range 5 {
		if a.next() != b.next() {
			t.Fatal("the same pinned seed gave two streams")
		}
	}
	c, d := seeded(4), seeded(3)
	if d.next() == c.next() {
		t.Error("two visuals share a stream")
	}
}

func TestASilentAssistantOnlySlotIsIdle(t *testing.T) {
	var fr framer
	x := Input{Replying: true, Dt: 100 * time.Millisecond}
	var f frame
	for i := range 30 {
		x.Now = time.Duration(i) * x.Dt
		f = fr.next(x)
	}
	if f.state != idle {
		t.Errorf("silent assistant slot is %q", f.state)
	}
	x.Speaker, x.Mic = loud(), loud()
	if f = fr.next(x); f.state != responding {
		t.Errorf("assistant speaking is %q", f.state)
	}
}

func TestQuietSpeechIsHeardAsSpeech(t *testing.T) {
	for _, raw := range []float32{0.06, 0.12, 0.4} {
		var fr framer
		var a analysis.Analysis
		a.Level = raw
		x := Input{Mic: a, Speaker: a, Dt: 50 * time.Millisecond}
		var f frame
		for i := range 200 {
			x.Now = time.Duration(i) * x.Dt
			f = fr.next(x)
		}
		if f.level < 0.55 || f.level > 0.75 {
			t.Errorf("steady speech at %.2f reads as %.2f", raw, f.level)
		}
	}
	var fr framer
	x := Input{Dt: 50 * time.Millisecond}
	var f frame
	for i := range 200 {
		x.Now = time.Duration(i) * x.Dt
		f = fr.next(x)
	}
	if f.level != 0 {
		t.Errorf("silence reads as %.2f", f.level)
	}
}

func TestAQuietVoiceShowsOverLoudMusic(t *testing.T) {
	var music, voice analysis.Analysis
	music.Level, voice.Level = 0.6, 0.05
	for i := range music.Wave {
		music.Wave[i] = 0.8 * float32(math.Sin(float64(i)*0.2))
		voice.Wave[i] = 0.05 * float32(math.Sin(float64(i)*1.3))
	}
	alone := Input{Mic: music, Speaker: music}.Voice()
	both := Input{Mic: voice, Speaker: music}.Voice()
	var moved float32
	for i := range both.Wave {
		moved = max(moved, both.Wave[i]-alone.Wave[i], alone.Wave[i]-both.Wave[i])
	}
	if moved < 0.3 {
		t.Errorf("a voice at a twelfth of the music's level moved the merged wave by %.2f", moved)
	}
}
