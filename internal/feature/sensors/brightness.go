package sensors

import "math"

// The range the panel is driven across when the brightness is automatic, before any bias.
//
// Never fully dark: a screen that goes black in an unlit room looks broken, and this device is
// something people glance at in the dark. Never fully bright either, since the top of the panel's
// range is uncomfortable in a living room.
const (
	MinBrightness = 8
	MaxBrightness = 92
)

// The room, in lux, between which the panel moves.
//
// A dark room reads near zero; an office is a few hundred; daylight through a window is
// thousands, and anything past that does not need a brighter screen.
const (
	darkRoom   = 2.0
	brightRoom = 800.0
)

// The slider's part in this.
//
// NeutralBias is the curve as it is tuned, so a device nobody has adjusted behaves as it did.
// Either end moves the whole curve by biasSpan points, and Floor is as dark as the panel is ever
// driven — below the curve's own bottom, because the complaint this answers is the panel being
// too bright at night, which is exactly where the curve is already at its floor.
const (
	NeutralBias = 50
	biasSpan    = 40
	Floor       = 2
)

// Brightness is how bright the panel should be for a room this lit, as a percentage, biased by
// what the brightness slider is set to.
//
// The bias moves the whole curve rather than scaling it. Scaling would leave the dark end nearly
// where it was, since the curve bottoms out at a level that is already small, and that is the end
// somebody turning the slider down is complaining about.
func Brightness(lux float64, bias int) int {
	return max(Floor, min(curve(lux)+shift(bias), 100))
}

// shift is what the slider moves the curve by, in percentage points.
func shift(bias int) int {
	bias = max(0, min(bias, 100))
	return (bias - NeutralBias) * biasSpan / NeutralBias
}

// curve is the unbiased level for a room this lit.
//
// Logarithmic, because perceived brightness is: the step from 2 lux to 20 matters as much as the
// step from 80 to 800, and a linear map spends almost all of its range on daylight.
func curve(lux float64) int {
	switch {
	case lux <= darkRoom:
		return MinBrightness
	case lux >= brightRoom:
		return MaxBrightness
	}

	span := math.Log(brightRoom) - math.Log(darkRoom)
	at := (math.Log(lux) - math.Log(darkRoom)) / span

	return MinBrightness + int(math.Round(at*float64(MaxBrightness-MinBrightness)))
}

const (
	holdRatio = 1.5
	holdLux   = 4.0
	holdFloor = 1.0
	rampStep  = 2
)

func hold(held, smoothed, raw float64) float64 {
	if smoothed > held*holdRatio+holdLux || smoothed < held/holdRatio-holdFloor {
		return raw
	}
	return held
}

func ramp(shown, want int) int {
	return shown + max(-rampStep, min(want-shown, rampStep))
}
