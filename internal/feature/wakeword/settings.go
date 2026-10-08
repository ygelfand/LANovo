package wakeword

import (
	"time"

	sharedtone "github.com/ygelfand/libcountertop/pkg/audio/tone"

	"github.com/ygelfand/LANovo/internal/config"
)

func saved(slot int) config.WakeWord { return config.Get().Wake.Slot(slot) }

func Threshold(slot int) float64 { return saved(slot).Threshold }

func Tones(slot int) bool { return !saved(slot).Tone.Silent() }

func ChimeLength(slot int) time.Duration {
	return sharedtone.Length(sharedtone.Wake(saved(slot).Tone))
}

func Delivery(slot int) config.Delivery { return saved(slot).Delivery }

func Buffer(slot int) time.Duration {
	return time.Duration(saved(slot).Buffer) * time.Millisecond
}

func FollowUp(slot int) time.Duration {
	return time.Duration(saved(slot).FollowUp) * time.Second
}

func MaxListen(slot int) time.Duration {
	return time.Duration(saved(slot).MaxListen) * time.Second
}

func MaxThink(slot int) time.Duration {
	return time.Duration(saved(slot).MaxThink) * time.Second
}

func Recordings(slot int) int { return saved(slot).Recordings }
