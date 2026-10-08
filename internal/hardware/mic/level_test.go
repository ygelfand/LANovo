package mic

import (
	"math"
	"testing"
)

func speech(dbfs float64) []int16 {
	frame := make([]int16, VoiceSamples)
	amp := math.Pow(10, dbfs/20) * fullScale * math.Sqrt2

	for i := range frame {
		frame[i] = int16(amp * math.Sin(2*math.Pi*300*float64(i)/Voice))
	}
	return frame
}

func levelOf(frame []int16) float64 {
	var sum float64
	for _, s := range frame {
		sum += float64(s) * float64(s)
	}
	return 20 * math.Log10(math.Sqrt(sum/float64(len(frame)))/fullScale)
}

func talk(l *leveler, room, voice float64, seconds float64) []int16 {
	var last []int16

	for range int(seconds * Voice / VoiceSamples / 20) {
		for range 15 {
			last = speech(voice)
			l.apply(last)
		}
		for range 5 {
			l.apply(speech(room))
		}
	}
	return last
}

func TestLevelerHoldsItsGainUnderPlayback(t *testing.T) {
	const voice = -45

	quiet := newLeveler()
	talk(quiet, -65, voice, 30)
	was := 20 * math.Log10(float64(quiet.gain))

	for _, echo := range []float64{-30, -24, -18} {
		for _, seconds := range []float64{0.5, 1, 3, 10} {
			l := newLeveler()
			talk(l, -65, voice, 30)

			l.atPlayback(true)
			for range int(seconds * Voice / VoiceSamples) {
				l.apply(speech(echo))
			}
			now := 20 * math.Log10(float64(l.gain))

			if short := was - now; short > 1 {
				t.Errorf("echo %+.0f dBFS for %.1fs: gain %+.1f -> %+.1f dB, short by %.1f",
					echo, seconds, was, now, short)
			}
		}
	}
}

func TestLevelerHoldsItsFloorUnderMusic(t *testing.T) {
	for _, music := range []float64{-30, -24, -18} {
		for _, seconds := range []float64{3, 30, 120} {
			l := newLeveler()
			talk(l, -65, -45, 30)
			was := 20 * math.Log10(float64(l.floor)/fullScale)

			l.atPlayback(true)
			for range int(seconds * Voice / VoiceSamples) {
				l.apply(speech(music))
			}

			if now := 20 * math.Log10(float64(l.floor)/fullScale); now-was > 1 {
				t.Errorf(
					"music %+.0f dBFS for %.0fs: floor %.1f -> %.1f dBFS",
					music,
					seconds,
					was,
					now,
				)
			}
		}
	}
}

func TestLevelerIsReadyWhenPlaybackStops(t *testing.T) {
	const voice = -45

	for _, quiet := range []bool{true, false} {
		l := newLeveler()
		talk(l, -65, voice, 30)
		was := 20 * math.Log10(float64(l.gain))

		l.atPlayback(true)
		for range int(30 * Voice / VoiceSamples) {
			l.apply(speech(-18))
		}
		l.atPlayback(false)

		if quiet {
			for range int(Voice / VoiceSamples) {
				l.apply(speech(-65))
			}
		} else {
			talk(l, -65, voice, 1)
		}

		if now := 20 * math.Log10(float64(l.gain)); was-now > 1 {
			t.Errorf("quiet=%t, a second after playback: gain %+.1f -> %+.1f dB", quiet, was, now)
		}
	}
}

func TestLevelerReachesTheTarget(t *testing.T) {
	l := newLeveler()

	got := levelOf(talk(l, -65, -45, 30))
	if math.Abs(got-targetDBFS) > 1.5 {
		t.Errorf("settled at %.1f dBFS, want %.1f", got, targetDBFS)
	}
}

func TestLevelerStopsAtTheCeiling(t *testing.T) {
	l := newLeveler()

	for range 500 {
		l.apply(speech(-70))
	}

	if gain := 20 * math.Log10(float64(l.gain)); gain > maxGainDB+0.5 {
		t.Errorf("gain reached %.1f dB, ceiling is %.1f", gain, maxGainDB)
	}
}

func TestLevelerLeavesLoudSpeechAlone(t *testing.T) {
	l := newLeveler()

	want := levelOf(speech(-12))
	got := levelOf(talk(l, -60, -12, 30))

	if math.Abs(got-want) > 1 {
		t.Errorf("level moved from %.1f to %.1f dBFS", want, got)
	}
}

func TestLevelerIgnoresSilence(t *testing.T) {
	l := newLeveler()

	for range 200 {
		l.apply(speech(-45))
	}
	spoken := l.gain

	for range 500 {
		l.apply(make([]int16, VoiceSamples))
	}

	if l.gain != spoken {
		t.Errorf("gain moved from %.2f to %.2f over silence", spoken, l.gain)
	}
}

func peaky(rmsDBFS, crestDB float64) []int16 {
	frame := speech(rmsDBFS)
	frame[len(frame)/2] = int16(min(math.Pow(10, (rmsDBFS+crestDB)/20)*fullScale, fullScale-1))
	return frame
}

func peakOf(frame []int16) float64 {
	var peak int32
	for _, s := range frame {
		peak = max(peak, int32(abs(s)))
	}
	return 20 * math.Log10(float64(peak)/fullScale)
}

func TestOneLoudFrameDoesNotTakeTheGainAway(t *testing.T) {
	l := newLeveler()
	talk(l, -65, -45, 30)
	settled := l.gain

	l.apply(speech(-3))
	for range 5 {
		l.apply(speech(-65))
	}
	if l.gain < settled*0.9 {
		t.Errorf(
			"one loud frame took the gain from %.1f dB to %.1f dB",
			20*math.Log10(float64(settled)),
			20*math.Log10(float64(l.gain)),
		)
	}
	if got := levelOf(talk(l, -65, -45, 1)); math.Abs(got-targetDBFS) > 2 {
		t.Errorf("speech after the loud frame came out at %.1f dBFS, want %.1f", got, targetDBFS)
	}
}

func TestLevelerDoesNotClipPeakySpeech(t *testing.T) {
	l := newLeveler()

	var worst float64 = -100
	for range 300 {
		frame := peaky(-35, 16)
		l.apply(frame)
		worst = max(worst, peakOf(frame))
	}

	if n := l.clipped.Load(); n != 0 {
		t.Errorf("clipped %d samples", n)
	}
	if worst > peakDBFS+0.5 {
		t.Errorf("peaked at %.1f dBFS, ceiling is %.1f", worst, peakDBFS)
	}
}

func TestLevelerDoesNotFollowTheRoom(t *testing.T) {
	for _, room := range []float64{-70, -55, -40} {
		l := newLeveler()
		start := l.gain

		for range 1000 {
			l.apply(speech(room))
		}

		if l.gain != start {
			t.Errorf("a room at %.0f dBFS moved the gain from %.2f to %.2f", room, start, l.gain)
		}
	}
}

func TestLevelerFollowsWhatStandsAboveTheRoom(t *testing.T) {
	for _, room := range []float64{-70, -55, -40} {
		l := newLeveler()

		for range 200 {
			l.apply(speech(room))
		}
		quiet := l.gain

		for range 200 {
			l.apply(speech(room + speechOverFloorDB + 6))
		}

		if l.gain == quiet {
			t.Errorf("speech %.0f dB above a room at %.0f dBFS did not move the gain",
				speechOverFloorDB+6, room)
		}
	}
}

func TestForgetClearsTheLearning(t *testing.T) {
	l := newLeveler()
	fresh := l.gain

	talk(l, -65, -45, 10)
	if l.gain == fresh {
		t.Fatal("gain never moved, so there is nothing to forget")
	}

	l.forget()
	if l.gain != fresh {
		t.Errorf("gain is %.2f after forgetting, want %.2f", l.gain, fresh)
	}
	if l.floor != fullScale {
		t.Errorf("floor is %.0f after forgetting, want it reset", l.floor)
	}
}

func TestLevelerFallsFasterThanItRises(t *testing.T) {
	l := newLeveler()
	if l.fall <= l.rise {
		t.Errorf("fall %.4f is not quicker than rise %.4f", l.fall, l.rise)
	}
}

func TestLevelIsRelativeToTheRoom(t *testing.T) {
	quiet, loud := newLeveler(), newLeveler()

	settle(quiet, -60, 200)
	settle(loud, -35, 200)

	quietVoice := level(quiet, -42, 40)
	loudVoice := level(loud, -17, 40)

	if math.Abs(quietVoice-loudVoice) > 0.1 {
		t.Errorf(
			"the same voice reads %.2f over a quiet room and %.2f over a loud one",
			quietVoice,
			loudVoice,
		)
	}

	if quietVoice < 0.2 || quietVoice > 0.6 {
		t.Errorf(
			"a voice 18 dB over the room reads %.2f, want it visible and short of the top",
			quietVoice,
		)
	}

	if talking := level(newRoom(t), -32, 40); talking < 0.6 {
		t.Errorf("speech 25 dB over the room reads %.2f, want most of the way up", talking)
	}
}

func newRoom(t *testing.T) *leveler {
	t.Helper()

	l := newLeveler()
	settle(l, -57, 200)
	return l
}

func TestLevelRestsAtZeroAndFallsBack(t *testing.T) {
	l := newLeveler()
	settle(l, -55, 200)

	if quiet := level(l, -55, 40); quiet > 0.05 {
		t.Errorf("a quiet room reads %.2f, want nothing", quiet)
	}

	spoke := level(l, -30, 20)
	if spoke < 0.5 {
		t.Fatalf("speech reads %.2f, want most of the way up", spoke)
	}

	if after := level(l, -55, 25); after > 0.3 {
		t.Errorf("half a second after speech the level is %.2f, want it mostly fallen", after)
	}

	if after := level(l, -55, 50); after > 0.05 {
		t.Errorf("a second and a half after speech the level is %.2f, want nothing", after)
	}
}

func TestLevelRisesFasterThanItFalls(t *testing.T) {
	l := newLeveler()
	settle(l, -55, 200)

	up := level(l, -30, 3)
	settle(l, -30, 40)
	down := level(l, -55, 3)

	if up < 0.2 {
		t.Errorf("three frames of speech reached %.2f, want it well up already", up)
	}
	if down < 0.5 {
		t.Errorf("three frames of quiet fell to %.2f, want it still most of the way up", down)
	}
}

func TestLevelIgnoresTheBandTheRingWhinesIn(t *testing.T) {
	quiet := speech(-55)

	for _, hz := range []float64{3000, 4500} {
		l := newLeveler()
		for range 200 {
			l.observe(quiet)
		}

		whine := tone(hz, -30)
		var loudest float64
		for range 60 {
			l.observe(whine)
			loudest = math.Max(loudest, float64(math.Float32frombits(l.level.Load())))
		}
		if loudest > 0.1 {
			t.Errorf("%.0f Hz at -30 dBFS drove the level to %.2f, want it ignored", hz, loudest)
		}
	}

	l := newLeveler()
	for range 200 {
		l.observe(quiet)
	}
	if got := level(l, -30, 60); got < 0.5 {
		t.Errorf("speech at -30 dBFS reads %.2f, want most of the way up", got)
	}
}

func TestLevelIgnoresSteadyRoomNoise(t *testing.T) {
	for _, hz := range []float64{60, 120} {
		l := newLeveler()

		hum := tone(hz, -45)
		for range 300 {
			l.observe(hum)
		}

		var loudest float64
		for range 100 {
			l.observe(tone(hz, -40))
			loudest = math.Max(loudest, float64(math.Float32frombits(l.level.Load())))
		}
		if loudest > 0.1 {
			t.Errorf(
				"%.0f Hz hum rising 5 dB drove the level to %.2f, want it ignored",
				hz,
				loudest,
			)
		}
	}
}

func tone(hz, dbfs float64) []int16 {
	frame := make([]int16, VoiceSamples)
	amp := math.Pow(10, dbfs/20) * fullScale * math.Sqrt2

	for i := range frame {
		frame[i] = int16(amp * math.Sin(2*math.Pi*hz*float64(i)/Voice))
	}
	return frame
}

func settle(l *leveler, dbfs float64, frames int) {
	for range frames {
		l.observe(speech(dbfs))
	}
}

func level(l *leveler, dbfs float64, frames int) float64 {
	settle(l, dbfs, frames)
	return float64(math.Float32frombits(l.level.Load()))
}
