package theme

import sharedtheme "github.com/ygelfand/libcountertop/pkg/display/theme"

var (
	Blue  = sharedtheme.RGB(0x0080f0)
	Navy  = sharedtheme.RGB(0x101f2e)
	Paper = sharedtheme.RGB(0xf7f9fb)

	Slate = sharedtheme.RGB(0x6b7b8c)

	Mist = sharedtheme.RGB(0xe4ebf2)
)

func Brand() sharedtheme.Theme {
	return sharedtheme.Theme{
		Name: "LANovo",
		Dark: false,

		Background: Mist,
		Surface:    Paper,

		Text:  Navy,
		Muted: Slate,

		Accent:  Blue,
		Accent2: sharedtheme.RGB(0x39a0ff),

		Success: sharedtheme.RGB(0x2e9e57),
		Warning: sharedtheme.RGB(0xb8791a),
		Danger:  sharedtheme.RGB(0xd13b3b),
	}
}
