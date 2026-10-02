package control

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/go-esphome-device/api"

	"github.com/ygelfand/LANovo/internal/feature/timer"
)

const harnessTimer = "ctl"

func countdown(_ *cobra.Command, args []string) error {
	switch args[0] {
	case "ring":
		timer.Get().Event(esphome.TimerEvent{Type: api.VoiceAssistantTimerEvent_VOICE_ASSISTANT_TIMER_FINISHED, TimerID: harnessTimer})
		return nil
	case "cancel":
		timer.Get().Event(esphome.TimerEvent{Type: api.VoiceAssistantTimerEvent_VOICE_ASSISTANT_TIMER_CANCELLED, TimerID: harnessTimer})
		return nil
	}
	seconds, err := strconv.Atoi(args[0])
	if err != nil || seconds <= 0 {
		return fmt.Errorf("usage: timer SECONDS [NAME] | timer ring | timer cancel")
	}
	timer.Get().Event(esphome.TimerEvent{
		Type:         api.VoiceAssistantTimerEvent_VOICE_ASSISTANT_TIMER_STARTED,
		TimerID:      harnessTimer,
		Name:         strings.Join(args[1:], " "),
		TotalSeconds: uint32(seconds),
		SecondsLeft:  uint32(seconds),
		IsActive:     true,
	})
	return nil
}
