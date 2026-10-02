package config

import "maps"

// Camera is what the camera has been set to, keyed by the setting names in
// internal/feature/livecam. Values are unvalidated; the table there does that.
type Camera struct {
	Settings map[string]string `json:"settings,omitempty"`
}

func defaultCamera() Camera { return Camera{Settings: map[string]string{}} }

func (c Camera) Set(name string) (string, bool) {
	v, ok := c.Settings[name]
	return v, ok
}

// CameraWriter changes what the camera is set to.
type CameraWriter struct{ st *Store }

// Set records one setting. An empty value forgets it.
func (w CameraWriter) Set(name, value string) error {
	return w.Put(map[string]string{name: value})
}

// Put records several settings in one write. An empty value forgets that one.
func (w CameraWriter) Put(values map[string]string) error {
	return w.st.Update(func(c *Config) {
		// Get hands this map to readers without copying it.
		settings := maps.Clone(c.Camera.Settings)
		if settings == nil {
			settings = map[string]string{}
		}
		for name, value := range values {
			if value == "" {
				delete(settings, name)
			} else {
				settings[name] = value
			}
		}
		c.Camera.Settings = settings
	})
}

func (w CameraWriter) Clear() error {
	return w.st.Update(func(c *Config) { c.Camera.Settings = map[string]string{} })
}
