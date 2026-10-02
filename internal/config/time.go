package config

// Time is how the device tells it.
type Time struct {
	// Home is the POSIX TZ string Home Assistant last sent, remembered so the clock is right from
	// the moment the panel lights rather than from the first connection.
	Home string `json:"zone"`

	// Chosen is an IANA name picked on the device, which outranks whatever Home Assistant sends.
	// Empty is the usual case and means follow the server.
	//
	// The gap this closes: ESPHome takes a timezone key at build time and has no runtime override,
	// so a device in a different zone from its server has nowhere to say so.
	Chosen string `json:"chosen"`
}

func defaultTime() Time { return Time{} }

// TimeWriter records how the device tells the time.
type TimeWriter struct{ st *Store }

// Home records what Home Assistant said, which is not necessarily what the device runs on.
func (w TimeWriter) Home(v string) error {
	return w.st.Update(func(c *Config) { c.Time.Home = v })
}

// Chosen sets the zone the device runs on whatever the server says. Empty gives it back.
func (w TimeWriter) Chosen(v string) error {
	return w.st.Update(func(c *Config) { c.Time.Chosen = v })
}
