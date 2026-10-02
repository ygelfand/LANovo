package touch

import "time"

// Kind is what a contact turned out to be.
type Kind int

const (
	// Tap is a contact that went down and came up without going anywhere.
	Tap Kind = iota

	// Swipe is a contact that traveled far enough to mean a direction.
	Swipe
)

func (k Kind) String() string {
	if k == Swipe {
		return "swipe"
	}
	return "tap"
}

// Edge is a side of the picture. Which side that is on the panel depends on the rotation, and
// touches already arrive in viewed coordinates, so nothing here has to think about it.
type Edge int

const (
	NoEdge Edge = iota
	Left
	Right
	Top
	Bottom
)

func (e Edge) String() string {
	switch e {
	case Left:
		return "left"
	case Right:
		return "right"
	case Top:
		return "top"
	case Bottom:
		return "bottom"
	}
	return "none"
}

// Gesture is one finger's whole journey, once it has lifted.
type Gesture struct {
	Kind Kind

	// From is the edge the contact started at, which is what an edge swipe is: a drawer is opened
	// by starting off the picture, not by a swipe in the middle of it.
	From Edge

	// Toward is the edge the contact headed for. A swipe in from the left goes toward the right.
	Toward Edge

	StartX, StartY int
	EndX, EndY     int

	Held time.Duration
}

// The fractions of the shorter side that decide what a contact was.
const (
	// edgeMargin is how close to a side a contact has to start to count as coming from it. Wide
	// enough to catch a finger placed on the bezel, narrow enough that a tap near the side of the
	// picture is still a tap.
	edgeMargin = 0.06

	// swipeDistance is how far a contact has to travel before it means a direction rather than an
	// imprecise tap.
	swipeDistance = 0.10
)

// Recognizer turns contacts into gestures. It is fed every contact and answers when one lifts.
type Recognizer struct {
	// W and H are the picture, in viewed coordinates.
	W, H int

	tracking map[int]*journey
}

// journey is where a contact started and when.
type journey struct {
	x, y int
	at   time.Time
	edge Edge
}

// NewRecognizer works on a picture of this size.
func NewRecognizer(w, h int) *Recognizer {
	return &Recognizer{W: w, H: h, tracking: map[int]*journey{}}
}

// Feed takes one contact and reports a gesture when that contact has finished.
//
// Tracked by id rather than slot: the kernel re-uses a slot for a new finger, and a journey that
// carried over would read as a swipe from wherever the last finger was.
func (r *Recognizer) Feed(c Contact) (Gesture, bool) {
	if r.tracking == nil {
		r.tracking = map[int]*journey{}
	}

	switch c.Phase {
	case Down:
		r.tracking[c.ID] = &journey{
			x: c.X, y: c.Y, at: at(c), edge: r.edgeOf(c.X, c.Y),
		}
	case Up:
		start, ok := r.tracking[c.ID]
		if !ok {
			return Gesture{}, false
		}
		delete(r.tracking, c.ID)

		return r.finish(start, c), true
	}
	return Gesture{}, false
}

// Forget drops what is being tracked, for a screen that has changed under the finger.
func (r *Recognizer) Forget() { r.tracking = map[int]*journey{} }

// finish decides what the journey was.
func (r *Recognizer) finish(start *journey, end Contact) Gesture {
	g := Gesture{
		From:   start.edge,
		StartX: start.x, StartY: start.y,
		EndX: end.X, EndY: end.Y,
		Held: at(end).Sub(start.at),
	}

	dx, dy := end.X-start.x, end.Y-start.y
	far := int(float64(min(r.W, r.H)) * swipeDistance)

	if abs(dx) < far && abs(dy) < far {
		g.Kind = Tap
		return g
	}

	g.Kind = Swipe
	if abs(dx) >= abs(dy) {
		g.Toward = Right
		if dx < 0 {
			g.Toward = Left
		}
		return g
	}

	g.Toward = Bottom
	if dy < 0 {
		g.Toward = Top
	}
	return g
}

// edgeOf is the side a position is against, if any. A corner belongs to whichever side it is
// closest to, so a contact is never ambiguous.
func (r *Recognizer) edgeOf(x, y int) Edge {
	margin := int(float64(min(r.W, r.H)) * edgeMargin)

	// The distance to each side, and the smallest wins.
	near := map[Edge]int{Left: x, Right: r.W - 1 - x, Top: y, Bottom: r.H - 1 - y}

	best, shortest := NoEdge, margin
	for edge, d := range near {
		if d < shortest || (d == shortest && best != NoEdge && edge < best) {
			best, shortest = edge, d
		}
	}
	return best
}

// at is when a contact happened, falling back to now for one that carries no time.
func at(c Contact) time.Time {
	if c.At.IsZero() {
		return time.Now()
	}
	return c.At
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
