package touch

import "time"

type Kind int

const (
	Tap Kind = iota

	Swipe
)

func (k Kind) String() string {
	if k == Swipe {
		return "swipe"
	}
	return "tap"
}

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

type Gesture struct {
	Kind Kind

	From Edge

	Toward Edge

	StartX, StartY int
	EndX, EndY     int

	Held time.Duration
}

const (
	edgeMargin = 0.06

	swipeDistance = 0.10
)

type Recognizer struct {
	W, H int

	tracking map[int]*journey
}

type journey struct {
	x, y int
	at   time.Time
	edge Edge
}

func NewRecognizer(w, h int) *Recognizer {
	return &Recognizer{W: w, H: h, tracking: map[int]*journey{}}
}

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

func (r *Recognizer) Forget() { r.tracking = map[int]*journey{} }

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

func (r *Recognizer) edgeOf(x, y int) Edge {
	margin := int(float64(min(r.W, r.H)) * edgeMargin)

	near := map[Edge]int{Left: x, Right: r.W - 1 - x, Top: y, Bottom: r.H - 1 - y}

	best, shortest := NoEdge, margin
	for edge, d := range near {
		if d < shortest || (d == shortest && best != NoEdge && edge < best) {
			best, shortest = edge, d
		}
	}
	return best
}

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
