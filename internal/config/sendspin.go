package config

// Sendspin is the room's part in whole-house audio.
type Sendspin struct {
	// Enabled is whether the room listens for a server. Off by default: it opens a port and
	// advertises itself, which is not something to start doing without being asked.
	Enabled bool `json:"enabled"`
}

func defaultSendspin() Sendspin { return Sendspin{} }

// SendspinWriter changes the room's part in whole-house audio.
type SendspinWriter struct{ st *Store }

func (w SendspinWriter) Enabled(v bool) error {
	return w.st.Update(func(c *Config) { c.Sendspin.Enabled = v })
}
