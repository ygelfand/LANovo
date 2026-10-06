package config

import (
	"fmt"
	"github.com/ygelfand/libcountertop/pkg/audio/ducking"
)

// Media is the saved attenuation applied to music under a voice turn.
type Media struct {
	DuckDB float64 `json:"duck_db"`
}

func defaultMedia() Media { return Media{DuckDB: ducking.LegacyDB} }

type MediaWriter struct{ st *Store }

func (w MediaWriter) DuckDB(db float64) error {
	if !ducking.Valid(db) {
		return fmt.Errorf("config: ducking must be between %d and %d dB", ducking.MinimumDB, ducking.MaximumDB)
	}
	return w.st.Update(func(c *Config) { c.Media.DuckDB = db })
}
