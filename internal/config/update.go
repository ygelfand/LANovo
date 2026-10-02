package config

// Update is which release stream the device follows, by label, and the last version it told Home
// Assistant about.
type Update struct {
	Channel     string `json:"channel"`
	LastVersion string `json:"last_version"`
}

func defaultUpdate() Update { return Update{Channel: "stable"} }

// UpdateWriter changes it.
type UpdateWriter struct{ st *Store }

func (w UpdateWriter) Channel(v string) error {
	return w.st.Update(func(c *Config) { c.Update.Channel = v })
}

func (w UpdateWriter) LastVersion(v string) error {
	return w.st.Update(func(c *Config) { c.Update.LastVersion = v })
}
