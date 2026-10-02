package config

// Visual is the live audio visual the device draws.
type Visual struct {
	// Kind names one of the visuals in internal/ui/visual.
	Kind string `json:"kind"`

	Label string `json:"label"`

	MaxFPS int `json:"max_fps"`

	Seed int `json:"seed"`
}

var MaxFPSSteps = []int{10, 15, 20, 30, 45, 60}

const (
	DefaultVisual = "orb"
	DefaultLabel  = "LANOVO"
	DefaultMaxFPS = 60
)

func defaultVisual() Visual {
	return Visual{Kind: DefaultVisual, Label: DefaultLabel, MaxFPS: DefaultMaxFPS}
}

// VisualWriter changes it.
type VisualWriter struct{ st *Store }

func (w VisualWriter) Kind(v string) error {
	return w.st.Update(func(c *Config) { c.Visual.Kind = v })
}

func (w VisualWriter) MaxFPS(v int) error {
	return w.st.Update(func(c *Config) { c.Visual.MaxFPS = v })
}

func (w VisualWriter) Seed(v int) error {
	return w.st.Update(func(c *Config) { c.Visual.Seed = v })
}

func (w VisualWriter) Label(v string) error {
	return w.st.Update(func(c *Config) { c.Visual.Label = v })
}
