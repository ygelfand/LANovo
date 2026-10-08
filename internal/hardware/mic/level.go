package mic

import (
	"log/slog"
	"math"
	"sync/atomic"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
)

const (
	targetDBFS = -23.0
	maxGainDB  = 27.0

	speechOverFloorDB = 12.0

	floorFall = 200 * time.Millisecond
	floorRise = 60 * time.Second

	// Speech peaks 12 to 18 dB above its own average.
	peakDBFS = -3.0

	// The vendor's own leveller carries a fixed 20 dB.
	startGainDB = 20.0

	fallTime = 500 * time.Millisecond
	riseTime = 5 * time.Second
)

const (
	levelLowHz  = 200
	levelHighHz = 900
	levelPoles  = 3

	levelTopDB = 30.0

	levelAttack  = 40 * time.Millisecond
	levelRelease = 180 * time.Millisecond
)

type leveler struct {
	gain float32

	least atomic.Uint32
	quiet atomic.Int32

	sounding bool

	peak atomic.Uint32

	floor float32

	fall, rise          float32
	floorDown, floorUp  float32
	target, ceiling     float32
	headroom, overFloor float32

	levelUp, levelDown float32

	lp                 [levelPoles]float32
	hp                 float32
	lpA, hpA           float32
	bandRMS, bandFloor float32

	published atomic.Uint32
	level     atomic.Uint32
	clipped   atomic.Uint64
}

func (l *leveler) forget() {
	l.gain = float32(math.Pow(10, startGainDB/20))
	l.floor, l.bandFloor = fullScale, fullScale
	l.publish()
}

func (l *leveler) publish() { l.published.Store(math.Float32bits(l.gain)) }

func newLeveler() *leveler {
	frame := float64(VoiceSamples) / float64(Voice)

	l := &leveler{
		gain:      float32(math.Pow(10, startGainDB/20)),
		floor:     fullScale,
		fall:      float32(1 - math.Exp(-frame/fallTime.Seconds())),
		rise:      float32(1 - math.Exp(-frame/riseTime.Seconds())),
		floorDown: float32(1 - math.Exp(-frame/floorFall.Seconds())),
		floorUp:   float32(1 - math.Exp(-frame/floorRise.Seconds())),
		target:    float32(math.Pow(10, targetDBFS/20) * fullScale),
		ceiling:   float32(math.Pow(10, maxGainDB/20)),
		headroom:  float32(math.Pow(10, peakDBFS/20) * fullScale),
		overFloor: float32(math.Pow(10, speechOverFloorDB/20)),

		levelUp:   float32(1 - math.Exp(-frame/levelAttack.Seconds())),
		levelDown: float32(1 - math.Exp(-frame/levelRelease.Seconds())),

		bandFloor: fullScale,
		lpA:       float32(1 - math.Exp(-2*math.Pi*levelHighHz/Voice)),
		hpA:       float32(1 - math.Exp(-2*math.Pi*levelLowHz/Voice)),
	}

	l.atGain(config.Get().Microphone.Gain)
	l.atSensitivity(config.Get().Microphone.Sensitivity)
	return l
}

func (l *leveler) atGain(db int) { l.least.Store(math.Float32bits(quietest(db))) }

func (l *leveler) atSensitivity(db int) { l.quiet.Store(int32(db)) }

func (l *leveler) atPlayback(on bool) { l.sounding = on }

const fullScale = 32768

func abs(s int16) int32 {
	if s < 0 {
		return -int32(s)
	}
	return int32(s)
}

func (l *leveler) observe(frame []int16) (rms, peak float32) {
	var sum, band float64
	for _, s := range frame {
		sum += float64(s) * float64(s)
		peak = max(peak, float32(abs(s)))

		v := float32(s)
		for i := range l.lp {
			l.lp[i] += (v - l.lp[i]) * l.lpA
			v = l.lp[i]
		}
		l.hp += (v - l.hp) * l.hpA
		v -= l.hp

		band += float64(v) * float64(v)
	}
	rms = float32(math.Sqrt(sum / float64(len(frame))))
	l.bandRMS = float32(math.Sqrt(band / float64(len(frame))))

	if !l.sounding {
		least := float32(math.Float32frombits(l.least.Load()))
		l.floor = follow(l.floor, rms, l.floorDown, l.floorUp, least)
		l.bandFloor = follow(l.bandFloor, l.bandRMS, l.floorDown, l.floorUp, least)
	}

	l.publishLevel(l.bandRMS)
	return rms, peak
}

const (
	quietestAtGain = 16.0
	quietestGainDB = config.DefaultMicGain
	quietestLimit  = 2.0
)

func quietest(db int) float32 {
	return float32(max(quietestAtGain*math.Pow(10, float64(db-quietestGainDB)/20), quietestLimit))
}

func follow(floor, rms, down, up, least float32) float32 {
	step := up
	if rms < floor {
		step = down
	}
	return max(floor+(rms-floor)*step, least)
}

func levelFrom(overDB, quiet, span float64) float64 {
	x := min(max((overDB-quiet)/span, 0), 1)

	return x * x * x * (x*(6*x-15) + 10)
}

func (l *leveler) publishLevel(rms float32) {
	over := 0.0
	if rms > l.bandFloor {
		over = 20 * math.Log10(float64(rms/l.bandFloor))
	}
	quiet := float64(l.quiet.Load())
	want := float32(levelFrom(over, quiet, levelTopDB-quiet))

	was := math.Float32frombits(l.level.Load())
	step := l.levelUp
	if want < was {
		step = l.levelDown
	}
	now := was + (want-was)*step
	l.level.Store(math.Float32bits(now))

	if now > math.Float32frombits(l.peak.Load()) {
		l.peak.Store(math.Float32bits(now))
	}
}

func (l *leveler) apply(frame []int16) {
	if len(frame) == 0 {
		return
	}
	rms, peak := l.observe(frame)

	if !l.sounding && rms > l.floor*l.overFloor {
		want := min(max(l.target/rms, 1), l.ceiling)

		step := l.rise
		if want < l.gain {
			step = l.fall
		}
		l.gain += (want - l.gain) * step
	}

	gain := l.gain
	if peak > 0 {
		gain = min(gain, l.headroom/peak)
	}

	l.publish()

	for i, s := range frame {
		switch v := float32(s) * gain; {
		case v > fullScale-1:
			frame[i] = fullScale - 1
			l.clipped.Add(1)
		case v < -fullScale:
			frame[i] = -fullScale
			l.clipped.Add(1)
		default:
			frame[i] = int16(v)
		}
	}
}

func (s *Mics) SetLeveling(on bool) {
	s.leveling.Store(on)
	slog.Info("microphone leveling", "on", on)
}

func (s *Mics) SetSensitivity(db int) {
	s.leveler.atSensitivity(db)
	slog.Info("room sensitivity", "over_floor_db", db)
}

func (s *Mics) Floor() float64 {
	return 20 * math.Log10(float64(s.leveler.bandFloor)/fullScale)
}

func (s *Mics) Peak() float64 {
	return float64(math.Float32frombits(s.leveler.peak.Swap(0)))
}

func (s *Mics) Leveled() float64 {
	return 20 * math.Log10(float64(math.Float32frombits(s.leveler.published.Load())))
}

func (s *Mics) Clipped() uint64 { return s.leveler.clipped.Load() }

func (s *Mics) Level() float64 {
	return float64(math.Float32frombits(s.leveler.level.Load()))
}
