package message

import (
	"testing"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func TestToneFallsBackToInfo(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Tone
	}{
		{"named", "warning", ToneWarning},
		{"alert", "alert", ToneAlert},
		{"success", "success", ToneSuccess},
		{"info", "info", ToneInfo},
		{"nothing given", "", ToneInfo},
		{"something else entirely", "urgent!!", ToneInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tone(tt.in); got != tt.want {
				t.Errorf("tone(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// A message outranks everything the device shows on its own, so how long it may hold the screen is
// bounded at both ends.
func TestSeconds(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want time.Duration
	}{
		{"unset takes the default", 0, DefaultSeconds * time.Second},
		{"negative takes the default", -5, DefaultSeconds * time.Second},
		{"a normal ask", 30, 30 * time.Second},
		{"the limit", MaxSeconds, MaxSeconds * time.Second},
		{"past the limit is clamped", 99999, MaxSeconds * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Hold(tt.in); got != tt.want {
				t.Errorf("Hold(%d) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// Every tone has to draw a color that is actually in the theme, or a message is invisible.
func TestEveryToneHasAColor(t *testing.T) {
	palette := theme.Default()

	for _, tone := range Tones() {
		c := tone.Color(palette)
		if c == palette.Surface {
			t.Errorf("%s is the same color as the card it sits on", tone)
		}
	}
}

func TestTonesAreLabelledDistinctly(t *testing.T) {
	seen := map[string]bool{}
	for _, tone := range Tones() {
		label := tone.Label()
		if label == "" {
			t.Errorf("%s has no label", tone)
		}
		if seen[label] {
			t.Errorf("two tones are labeled %q", label)
		}
		seen[label] = true
	}
}

func TestActions(t *testing.T) {
	acts := Get().Actions()
	if len(acts) != 2 {
		t.Fatalf("%d actions, want show and clear", len(acts))
	}

	var found bool
	for _, a := range acts {
		if a.Name != "show_message" {
			continue
		}
		found = true

		// The zero call carries no arguments, which is what a caller who forgot the body sends.
		if _, err := a.Run(esphome.Call{}); err == nil {
			t.Error("show_message with no body was accepted")
		}
	}
	if !found {
		t.Error("there is no show_message action")
	}
}

// Touching the screen when nothing is up must do nothing at all, rather than hiding a message
// that was never shown.
func TestDismissWithNothingShowing(t *testing.T) {
	m := Get()

	m.mu.Lock()
	m.timer = nil
	m.mu.Unlock()

	m.dismiss(0)
}
