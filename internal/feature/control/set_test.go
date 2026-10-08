package control

import (
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

func TestEverySettingReportsItsValue(t *testing.T) {
	cfg := config.Defaults()

	seen := map[string]bool{}
	for _, s := range settings() {
		if s.Name == "" {
			t.Error("a setting has no name")
		}
		if seen[s.Name] {
			t.Errorf("%q is listed twice", s.Name)
		}
		seen[s.Name] = true

		if s.Read == nil || s.Write == nil {
			t.Fatalf("%q is not wired up", s.Name)
			continue
		}
		if s.Read(cfg) == "" {
			t.Errorf("%q reads as nothing on a fresh device", s.Name)
		}
	}
}

func TestSetRefusesASettingThatDoesNotExist(t *testing.T) {
	_, err := set([]string{"clock.colour"})

	if err == nil {
		t.Fatal("an unknown setting was accepted")
	}
	if !strings.Contains(err.Error(), "no such setting") {
		t.Errorf("unhelpful error: %v", err)
	}
}
