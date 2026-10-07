package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type Weather = schema.Weather
type WeatherLook = schema.WeatherLook

const (
	WeatherCompact = schema.WeatherCompact
	WeatherStack   = schema.WeatherStack
	WeatherIcon    = schema.WeatherIcon
	WeatherWords   = schema.WeatherWords
	WeatherCard    = schema.WeatherCard
	WeatherDetail  = schema.WeatherDetail
)

var WeatherLooks = schema.WeatherLooks
var defaultWeather = schema.DefaultWeather

type WeatherWriter struct{ st *Store }

func (w WeatherWriter) Entity(v string) error {
	return w.st.Update(func(c *Config) { c.Weather.Entity = v })
}

func (w WeatherWriter) Look(v WeatherLook) error {
	return w.st.Update(func(c *Config) { c.Weather.Look = v })
}

func (w WeatherWriter) Dashboard(v bool) error {
	return w.st.Update(func(c *Config) { c.Weather.Dashboard = v })
}

func (w WeatherWriter) Idle(v bool) error {
	return w.st.Update(func(c *Config) { c.Weather.Idle = v })
}

func (w WeatherWriter) Animate(v bool) error {
	return w.st.Update(func(c *Config) { c.Weather.Animate = v })
}

func (w WeatherWriter) Themed(v bool) error {
	return w.st.Update(func(c *Config) { c.Weather.Themed = v })
}

func (w WeatherWriter) Position(v Position) error {
	return w.st.Update(func(c *Config) { c.Weather.Position = v })
}

func (w WeatherWriter) Align(v Align) error {
	return w.st.Update(func(c *Config) { c.Weather.Align = v })
}

func (w WeatherWriter) Size(v Size) error {
	return w.st.Update(func(c *Config) { c.Weather.Size = v })
}
