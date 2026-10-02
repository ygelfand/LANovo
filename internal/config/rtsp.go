package config

// RTSP is whether the camera is served over RTSP.
type RTSP struct {
	Enabled bool `json:"enabled"`
}

func defaultRTSP() RTSP { return RTSP{} }

// RTSPWriter changes it.
type RTSPWriter struct{ st *Store }

func (w RTSPWriter) Enabled(v bool) error {
	return w.st.Update(func(c *Config) { c.RTSP.Enabled = v })
}
