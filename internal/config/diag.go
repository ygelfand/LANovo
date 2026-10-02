package config

// Diag is how the device reports on itself.
type Diag struct {
	// Interval is how often the readings that drift are taken, in seconds.
	Interval int `json:"interval"`
}

// Sixty, because these are trends rather than events: nothing here is worth a graph at finer
// resolution, and every sample is a handful of file reads.
const DefaultInterval = 60

func defaultDiag() Diag { return Diag{Interval: DefaultInterval} }

// DiagWriter changes how the device reports on itself.
type DiagWriter struct{ st *Store }

func (w DiagWriter) Interval(v int) error {
	return w.st.Update(func(c *Config) { c.Diag.Interval = v })
}
