package config

import "github.com/ygelfand/LANovo/internal/lib/say"

type Weather struct {
	Entity    string      `json:"entity"`
	Look      WeatherLook `json:"look"`
	Dashboard bool        `json:"dashboard"`
	Idle      bool        `json:"idle"`
	Animate   bool        `json:"animate"`
	Themed    bool        `json:"themed"`
	Position  Position    `json:"position"`
	Align     Align       `json:"align"`
	Size      Size        `json:"size"`
}

type WeatherLook string

const (
	WeatherCompact WeatherLook = "compact"
	WeatherStack   WeatherLook = "stack"
	WeatherIcon    WeatherLook = "icon"
	WeatherWords   WeatherLook = "words"
	WeatherCard    WeatherLook = "card"
	WeatherDetail  WeatherLook = "detail"
)

func WeatherLooks() []WeatherLook {
	return []WeatherLook{WeatherCompact, WeatherStack, WeatherIcon, WeatherWords, WeatherCard, WeatherDetail}
}

func (l WeatherLook) Label() string { return say.T("weather.look." + string(l)) }

func defaultWeather() Weather {
	return Weather{Look: WeatherCompact, Dashboard: true, Animate: true, Position: PositionTop, Align: AlignRight, Size: SizeSmall}
}

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
