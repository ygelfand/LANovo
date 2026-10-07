package touch

import (
	"testing"
	"time"
)

// The picture as this device shows it, landscape.
const (
	screenW = 1920
	screenH = 1200
)

func down(id, x, y int) Contact {
	return Contact{ID: id, X: x, Y: y, Phase: Down, At: time.Now()}
}

func up(id, x, y int) Contact {
	return Contact{ID: id, X: x, Y: y, Phase: Up, At: time.Now()}
}

// journey feeds a whole contact and returns what it turned out to be.
func journeyOf(t *testing.T, r *Recognizer, id, x1, y1, x2, y2 int) Gesture {
	t.Helper()

	if _, ok := r.Feed(down(id, x1, y1)); ok {
		t.Fatal("a gesture was reported before the finger lifted")
	}
	r.Feed(Contact{ID: id, X: (x1 + x2) / 2, Y: (y1 + y2) / 2, Phase: Move, At: time.Now()})

	g, ok := r.Feed(up(id, x2, y2))
	if !ok {
		t.Fatal("lifting reported no gesture")
	}
	return g
}

func TestTap(t *testing.T) {
	r := NewRecognizer(screenW, screenH)

	g := journeyOf(t, r, 1, 900, 600, 903, 604)
	if g.Kind != Tap {
		t.Errorf("a finger that went nowhere is a %v, want a tap", g.Kind)
	}
}

func TestSwipeDirections(t *testing.T) {
	tests := []struct {
		name           string
		x1, y1, x2, y2 int
		want           Edge
	}{
		{"rightward", 200, 600, 900, 600, Right},
		{"leftward", 900, 600, 200, 600, Left},
		{"downward", 900, 200, 900, 900, Bottom},
		{"upward", 900, 900, 900, 200, Top},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRecognizer(screenW, screenH)

			g := journeyOf(t, r, 1, tt.x1, tt.y1, tt.x2, tt.y2)
			if g.Kind != Swipe {
				t.Fatalf("got a %v, want a swipe", g.Kind)
			}
			if g.Toward != tt.want {
				t.Errorf("headed %v, want %v", g.Toward, tt.want)
			}
		})
	}
}

// A drawer is opened by starting off the picture, so where the finger began is what matters.
func TestSwipeFromAnEdge(t *testing.T) {
	tests := []struct {
		name string
		x, y int
		want Edge
	}{
		{"hard against the left", 0, 600, Left},
		{"just inside the left", 20, 600, Left},
		{"hard against the right", screenW - 1, 600, Right},
		{"the top", 900, 2, Top},
		{"the bottom", 900, screenH - 1, Bottom},
		{"the middle is no edge", 900, 600, NoEdge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRecognizer(screenW, screenH)

			if got := r.edgeOf(tt.x, tt.y); got != tt.want {
				t.Errorf("edgeOf(%d, %d) = %v, want %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

// A swipe in from the left edge is the drawer gesture, and has to report both halves.
func TestEdgeSwipeCarriesBothEnds(t *testing.T) {
	r := NewRecognizer(screenW, screenH)

	g := journeyOf(t, r, 1, 4, 600, 700, 610)

	if g.Kind != Swipe {
		t.Fatalf("got a %v, want a swipe", g.Kind)
	}
	if g.From != Left {
		t.Errorf("came from %v, want the left edge", g.From)
	}
	if g.Toward != Right {
		t.Errorf("headed %v, want right", g.Toward)
	}
}

// A tap near the side of the picture is still a tap, or every touch near an edge opens a drawer.
func TestTapAtAnEdgeIsStillATap(t *testing.T) {
	r := NewRecognizer(screenW, screenH)

	g := journeyOf(t, r, 1, 4, 600, 8, 604)
	if g.Kind != Tap {
		t.Errorf("a short contact at the edge is a %v, want a tap", g.Kind)
	}
	if g.From != Left {
		t.Errorf("it started at %v, want the left edge recorded anyway", g.From)
	}
}

// The kernel re-uses a slot for a new finger, so a journey tracked by slot would read the new
// finger as a swipe from wherever the old one was. Tracking is by id.
func TestJourneysAreTrackedByID(t *testing.T) {
	r := NewRecognizer(screenW, screenH)

	// Two fingers in the same slot, one after the other, at opposite sides.
	r.Feed(Contact{Slot: 0, ID: 1, X: 100, Y: 600, Phase: Down, At: time.Now()})
	r.Feed(Contact{Slot: 0, ID: 1, X: 100, Y: 600, Phase: Up, At: time.Now()})

	r.Feed(Contact{Slot: 0, ID: 2, X: 1800, Y: 600, Phase: Down, At: time.Now()})
	g, ok := r.Feed(Contact{Slot: 0, ID: 2, X: 1805, Y: 604, Phase: Up, At: time.Now()})

	if !ok {
		t.Fatal("the second finger reported no gesture")
	}
	if g.Kind != Tap {
		t.Errorf(
			"the second finger came out as a %v, want a tap from where it actually started",
			g.Kind,
		)
	}
	if g.StartX != 1800 {
		t.Errorf("it started at %d, want 1800", g.StartX)
	}
}

// Two fingers at once are two journeys, not one.
func TestTwoFingersAreIndependent(t *testing.T) {
	r := NewRecognizer(screenW, screenH)

	r.Feed(down(1, 100, 300))
	r.Feed(down(2, 1800, 900))

	a, ok := r.Feed(up(1, 800, 310))
	if !ok || a.Kind != Swipe || a.Toward != Right {
		t.Errorf("the first finger came out as %+v", a)
	}

	b, ok := r.Feed(up(2, 1805, 905))
	if !ok || b.Kind != Tap {
		t.Errorf("the second finger came out as %+v", b)
	}
}

// A lift with no matching down says nothing, which is what happens when a finger was already on
// the glass before this recognizer started.
func TestLiftWithoutADown(t *testing.T) {
	r := NewRecognizer(screenW, screenH)

	if _, ok := r.Feed(up(9, 100, 100)); ok {
		t.Error("a lift with no down reported a gesture")
	}
}

// A screen that changed under the finger drops what it was tracking.
func TestForget(t *testing.T) {
	r := NewRecognizer(screenW, screenH)
	r.Feed(down(1, 100, 600))

	r.Forget()

	if _, ok := r.Feed(up(1, 900, 600)); ok {
		t.Error("a gesture survived Forget")
	}
}

// Portrait is the same code with the sides swapped, which is the point of working in viewed
// coordinates.
func TestWorksInPortrait(t *testing.T) {
	r := NewRecognizer(screenH, screenW)

	g := journeyOf(t, r, 1, 4, 900, 700, 910)
	if g.From != Left || g.Toward != Right {
		t.Errorf("portrait swipe came out from %v toward %v", g.From, g.Toward)
	}
}

// The recognizer is rebuilt when the device turns, because the picture changes shape and a
// journey measured against the old one would report the wrong edges.
func TestRecognizerFollowsTheRotation(t *testing.T) {
	s := &Screen{}

	// Whatever the display says now, one contact establishes a recognizer.
	s.recognize(down(1, 10, 10))

	s.mu.Lock()
	first, rot := s.rec, s.rot
	s.mu.Unlock()

	if first == nil {
		t.Fatal("no recognizer was built")
	}

	// Pretend the device turned since.
	s.mu.Lock()
	s.rot = rot + 90
	s.mu.Unlock()

	s.recognize(down(2, 10, 10))

	s.mu.Lock()
	second := s.rec
	s.mu.Unlock()

	if second == first {
		t.Error("the recognizer survived a rotation")
	}
}

// The recognizer is sized in viewed coordinates, so at the mounted rotation it is the landscape
// picture rather than the portrait panel.
func TestRecognizerIsSizedInViewedCoordinates(t *testing.T) {
	s := &Screen{}
	s.recognize(down(1, 10, 10))

	s.mu.Lock()
	rec, rot := s.rec, s.rot
	s.mu.Unlock()

	w, h := rot.Size(1200, 1920)
	if rec.W != w || rec.H != h {
		t.Errorf("the recognizer is %dx%d, want %dx%d at %v", rec.W, rec.H, w, h, rot)
	}
}
