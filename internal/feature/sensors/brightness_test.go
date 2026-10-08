package sensors

import "testing"

func TestBrightnessStaysInRange(t *testing.T) {
	for _, bias := range []int{-50, 0, NeutralBias, 100, 150} {
		for _, lux := range []float64{-100, -1, 0, 0.5, 2, 10, 100, 800, 10000, 1e9} {
			got := Brightness(lux, bias)
			if got < Floor || got > 100 {
				t.Errorf("Brightness(%v, %d) = %d, outside %d..100", lux, bias, got, Floor)
			}
		}
	}
}

func TestTheMiddleOfTheSliderIsTheCurve(t *testing.T) {
	for _, lux := range []float64{0, 2, 10, 100, 800, 10000} {
		if got, want := Brightness(lux, NeutralBias), curve(lux); got != want {
			t.Errorf("Brightness(%v, neutral) = %d, want the curve's %d", lux, got, want)
		}
	}
}

func TestBrightnessEnds(t *testing.T) {
	if got := Brightness(0, NeutralBias); got != MinBrightness {
		t.Errorf("an unlit room gave %d, want %d", got, MinBrightness)
	}
	if got := Brightness(darkRoom, NeutralBias); got != MinBrightness {
		t.Errorf("a dark room gave %d, want %d", got, MinBrightness)
	}
	if got := Brightness(brightRoom, NeutralBias); got != MaxBrightness {
		t.Errorf("a bright room gave %d, want %d", got, MaxBrightness)
	}
	if got := Brightness(100000, NeutralBias); got != MaxBrightness {
		t.Errorf("full daylight gave %d, want %d", got, MaxBrightness)
	}
}

func TestBrightnessRisesWithTheRoom(t *testing.T) {
	for _, bias := range []int{0, 25, NeutralBias, 75, 100} {
		last := -1
		for lux := 0.0; lux < 1200; lux += 0.5 {
			got := Brightness(lux, bias)
			if got < last {
				t.Fatalf(
					"bias %d: Brightness(%v) = %d, below the %d before it",
					bias,
					lux,
					got,
					last,
				)
			}
			last = got
		}
	}
}

func TestTurningTheSliderDownDimsEveryRoom(t *testing.T) {
	for _, lux := range []float64{0, 2, 10, 100, 800, 10000} {
		dim := Brightness(lux, 0)
		same := Brightness(lux, NeutralBias)
		bright := Brightness(lux, 100)

		if dim >= same {
			t.Errorf(
				"%v lux: the slider down gave %d, no dimmer than the middle's %d",
				lux,
				dim,
				same,
			)
		}
		if bright <= same {
			t.Errorf(
				"%v lux: the slider up gave %d, no brighter than the middle's %d",
				lux,
				bright,
				same,
			)
		}
	}
}

func TestTheDarkestTheSliderReachesIsStillLit(t *testing.T) {
	if got := Brightness(0, 0); got != Floor {
		t.Errorf("an unlit room with the slider down gave %d, want the floor of %d", got, Floor)
	}
	if Floor <= 0 {
		t.Error("the floor is off")
	}
}

func TestBrightnessSpendsItsRangeOnRoomsNotDaylight(t *testing.T) {
	dim := Brightness(20, NeutralBias)
	lit := Brightness(200, NeutralBias)

	if dim <= MinBrightness+5 {
		t.Errorf("a dim room gave %d, barely above the floor of %d", dim, MinBrightness)
	}
	if lit >= MaxBrightness-5 {
		t.Errorf("an office gave %d, already at the ceiling of %d", lit, MaxBrightness)
	}
	if lit <= dim {
		t.Errorf("an office (%d) is no brighter than a dim room (%d)", lit, dim)
	}

	if step := lit - dim; step < 10 || step > 60 {
		t.Errorf("ten times the light moved the panel by %d, want something in between", step)
	}
}

func TestTheScreensOwnGlowInADarkRoomDoesNotMoveTheLevel(t *testing.T) {
	held := 2.0
	for i := range 40 {
		lux := 2.0
		if i%2 == 1 {
			lux = 6
		}
		held = hold(held, lux, lux)
	}
	if held != 2 {
		t.Errorf("held %v after the panel lit its own sensor", held)
	}
}

func TestARealChangeInTheRoomIsTaken(t *testing.T) {
	if got := hold(2, 40, 40); got != 40 {
		t.Errorf("lights on held at %v", got)
	}
	if got := hold(300, 20, 20); got != 20 {
		t.Errorf("lights off held at %v", got)
	}
}

func TestADimRoomGoingDarkIsTaken(t *testing.T) {
	if got := hold(5, 0, 0); got != 0 {
		t.Errorf("lights off in a dim room held at %v", got)
	}
}

func TestAfterABrightSpikeTheLevelComesBackToTheRoom(t *testing.T) {
	following, held := 5.0, 5.0
	for i := range 80 {
		lux := 5.0
		if i < 20 {
			lux = 400
		}
		following += settle * (lux - following)
		held = hold(held, following, lux)
	}
	if got, want := Brightness(held, NeutralBias), Brightness(5, NeutralBias); got != want {
		t.Errorf(
			"after a flashlight the panel settled at %d, the room calls for %d (held %.1f lux)",
			got,
			want,
			held,
		)
	}
}

func TestTheLevelRampsRatherThanJumps(t *testing.T) {
	shown, steps := 8, 0
	for shown != 40 {
		next := ramp(shown, 40)
		if next-shown > rampStep {
			t.Fatalf("stepped %d at once", next-shown)
		}
		shown = next
		steps++
	}
	if steps != 16 {
		t.Errorf("%d steps from 8 to 40", steps)
	}
}
