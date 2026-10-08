package config

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

func TestADeviceWithNoSettingStillChimes(t *testing.T) {
	c := Defaults()

	if c.Feedback.Chime.Silent() {
		t.Error("a fresh device is silent")
	}
	if c.Feedback.Chime != schema.DefaultChime {
		t.Errorf("a fresh device is set to %v, want %v", c.Feedback.Chime, schema.DefaultChime)
	}
}

func TestTheChimeSurvivesARestart(t *testing.T) {
	path := fresh(t)

	if err := Set().Feedback().Chime(schema.ChimeNone); err != nil {
		t.Fatal(err)
	}

	Use(path)
	if got := Get().Feedback.Chime; got != schema.ChimeNone {
		t.Errorf("came back as %v, want None", got)
	}
}
