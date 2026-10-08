package config

type Diag struct {
	Interval int `json:"interval"`
}

const DefaultInterval = 60

func defaultDiag() Diag { return Diag{Interval: DefaultInterval} }

type DiagWriter struct{ st *Store }

func (w DiagWriter) Interval(v int) error {
	return w.st.Update(func(c *Config) { c.Diag.Interval = v })
}
