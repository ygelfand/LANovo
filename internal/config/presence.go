package config

type Presence struct {
	Wake  bool `json:"wake"`
	Range int  `json:"range"`
}

const (
	PresenceRangeMin     = 1
	PresenceRangeMax     = 5
	DefaultPresenceRange = 3
)

func defaultPresence() Presence { return Presence{Range: DefaultPresenceRange} }

type PresenceWriter struct{ st *Store }

func (w PresenceWriter) Set(p Presence) error {
	return w.st.Update(func(c *Config) { c.Presence = p })
}
