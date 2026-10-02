package theme

// The mark's own colors, sampled from it: the blue is over half of what it draws, the navy
// nearly a third, and the rest is close to white.
//
// These are the device's identity rather than a theme. A screen that belongs to LANovo itself —
// starting up, asking to be set up — uses them whatever theme is chosen, so it looks like the
// same product every time.
var (
	Blue  = rgb(0x0080f0)
	Navy  = rgb(0x101f2e)
	Paper = rgb(0xf7f9fb)

	// Slate is for the words that support the ones being read.
	Slate = rgb(0x6b7b8c)

	// Mist is the ground a card sits on: paper taken a few steps toward the navy, so the card is
	// the brighter of the two and reads as being on top of it.
	Mist = rgb(0xe4ebf2)
)

// Brand is the palette for screens the device owns.
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
