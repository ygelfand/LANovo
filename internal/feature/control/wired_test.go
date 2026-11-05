package control

import (
	"reflect"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/settings/settingtest"

	"github.com/ygelfand/LANovo/internal/config"
)

var coverage = settingtest.Shared.With(settingtest.Coverage{
	Rows: map[string]string{
		"Microphone.VisualizerLift": "visual.lift",
		"Microphone.Gain":           "microphone.gain",
		"Microphone.Leveling":       "microphone.leveling",
		"Microphone.Denoise":        "microphone.denoise",
		"Microphone.Sensitivity":    "microphone.sensitivity",
		"Presence.Wake":             "presence.wake",
		"Presence.Range":            "presence.range",
	},
	ByHand: map[string]string{
		"Device.Name":  "told to the process at start-up, never written to the file",
		"Device.Addr":  "the same",
		"Device.Model": "the same, read from the hardware",

		"Volume.Main":     "the volume command sets these, by stream",
		"Volume.Media":    "the volume command",
		"Volume.Alerts":   "the volume command",
		"Volume.Voice":    "the volume command",
		"Volume.Feedback": "the volume command",
	},
})

func TestEverySettingIsReachedOrAccountedFor(t *testing.T) {
	coverage.Accounts(t, reflect.TypeFor[config.Config]())
}

func TestEverySettingSaysWhatItChanges(t *testing.T) {
	var names []string
	for _, s := range settings() {
		names = append(names, s.Name)
	}
	coverage.Names(t, names)
}
