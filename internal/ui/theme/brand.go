package theme

var (
	Blue  = rgb(0x0080f0)
	Navy  = rgb(0x101f2e)
	Paper = rgb(0xf7f9fb)

	Slate = rgb(0x6b7b8c)

	Mist = rgb(0xe4ebf2)
)

func Brand() Theme {
	return Theme{
		Name: "LANovo",
		Dark: false,

		Background: Mist,
		Surface:    Paper,

		Text:  Navy,
		Muted: Slate,

		Accent:  Blue,
		Accent2: rgb(0x39a0ff),

		Success: rgb(0x2e9e57),
		Warning: rgb(0xb8791a),
		Danger:  rgb(0xd13b3b),
	}
}
