package config

import (
	"fmt"
	"github.com/ygelfand/libcountertop/pkg/audio/ducking"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

type Media = schema.Media

const DefaultDuckDB = ducking.DefaultDB

var defaultMedia = schema.DefaultMedia

type MediaWriter struct{ st *Store }

func (w MediaWriter) DuckDB(db float64) error {
	if !ducking.Valid(db) {
		return fmt.Errorf(
			"config: ducking must be between %d and %d dB",
			ducking.MinimumDB,
			ducking.MaximumDB,
		)
	}
	return w.st.Update(func(c *Config) { c.Media.DuckDB = db })
}
