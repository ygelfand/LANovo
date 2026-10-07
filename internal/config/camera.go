package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"
import "maps"

type Camera = schema.Camera

var defaultCamera = schema.DefaultCamera

type CameraWriter struct{ st *Store }

func (w CameraWriter) Set(name, value string) error {
	return w.Put(map[string]string{name: value})
}

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
