package sensors

import "math"

const (
	MinBrightness = 8
	MaxBrightness = 92
)

const (
	darkRoom   = 2.0
	brightRoom = 800.0
)

const (
	NeutralBias = 50
	biasSpan    = 40
	Floor       = 2
)

func Brightness(lux float64, bias int) int {
	return max(Floor, min(curve(lux)+shift(bias), 100))
}

func shift(bias int) int {
	bias = max(0, min(bias, 100))
	return (bias - NeutralBias) * biasSpan / NeutralBias
}

// Perceived brightness is logarithmic.
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
