package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type Ink = schema.Ink

const (
	InkTheme   = schema.InkTheme
	InkAmber   = schema.InkAmber
	InkRed     = schema.InkRed
	InkGreen   = schema.InkGreen
	InkCyan    = schema.InkCyan
	InkBlue    = schema.InkBlue
	InkViolet  = schema.InkViolet
	InkPink    = schema.InkPink
	DefaultInk = schema.DefaultInk
)

var Inks = schema.Inks
