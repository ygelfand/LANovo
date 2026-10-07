package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type Screen = schema.Screen
type ScreenMode = schema.ScreenMode
type HourFormat = schema.HourFormat
type KeyboardSize = schema.KeyboardSize

const (
	DefaultBacklight  = schema.DefaultBacklight
	DefaultScreenMode = schema.DefaultScreenMode
	DefaultHourFormat = schema.DefaultHourFormat
	DefaultMarks      = schema.DefaultMarks
	UISizeMini        = schema.UISizeMini
	UISizeCompact     = schema.UISizeCompact
	UISizeMedium      = schema.UISizeMedium
	UISizeLarge       = schema.UISizeLarge
	UISizeXLarge      = schema.UISizeXLarge
	ModeAuto          = schema.ModeAuto
	ModeManual        = schema.ModeManual
	TwentyFourHour    = schema.TwentyFourHour
	TwelveHour        = schema.TwelveHour
	KeyboardCompact   = schema.KeyboardCompact
	KeyboardStandard  = schema.KeyboardStandard
	KeyboardLarge     = schema.KeyboardLarge
	DefaultTheme      = schema.DefaultTheme
)

var ScreenSizes = schema.ScreenSizes
var ScreenModes = schema.ScreenModes
var HourFormats = schema.HourFormats
var KeyboardSizes = schema.KeyboardSizes
var defaultScreen = schema.DefaultScreen

type ScreenWriter struct{ st *Store }

func (w ScreenWriter) All(v Screen) error {
	return w.st.Update(func(c *Config) { c.Screen = v })
}

func (w ScreenWriter) Marks(v bool) error {
	return w.st.Update(func(c *Config) { c.Screen.Marks = v })
}

func (w ScreenWriter) Backlight(v int) error {
	return w.st.Update(func(c *Config) { c.Screen.Backlight = v })
}

func (w ScreenWriter) Mode(v ScreenMode) error {
	return w.st.Update(func(c *Config) { c.Screen.Mode = v })
}

func (w ScreenWriter) Theme(v string) error {
	return w.st.Update(func(c *Config) { c.Screen.Theme = v })
}

func (w ScreenWriter) Style(v string) error {
	return w.st.Update(func(c *Config) { c.Screen.Style = v })
}

func (w ScreenWriter) Size(v string) error {
	return w.st.Update(func(c *Config) { c.Screen.Size = v })
}

func (w ScreenWriter) Hours(v HourFormat) error {
	return w.st.Update(func(c *Config) { c.Screen.Hours = v })
}

func (w ScreenWriter) Keyboard(v KeyboardSize) error {
	return w.st.Update(func(c *Config) { c.Screen.Keyboard = v })
}

func (w ScreenWriter) Drawer(v Edge) error {
	return w.st.Update(func(c *Config) { c.Screen.Drawer = v })
}

func (w ScreenWriter) Volume(v Edge) error {
	return w.st.Update(func(c *Config) { c.Screen.Volume = v })
}

func (w ScreenWriter) Language(v string) error {
	return w.st.Update(func(c *Config) { c.Screen.Language = v })
}
