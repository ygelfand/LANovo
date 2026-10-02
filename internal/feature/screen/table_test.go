package screen

import (
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

// The identifiers Home Assistant already knows these by. They predate the table, which would have
// generated screen_backlight and screen_auto, and changing one orphans the entity it used to name
// along with whatever automation referred to it.
func TestTheEntityIdentifiersDoNotMove(t *testing.T) {
	want := map[string]string{
		"backlight": "backlight",
		"auto":      "screen_mode",
		"theme":     "theme",
		"style":     "screen_style",
		"size":      "screen_size",
		"drawer":    "drawer_edge",
	}

	rows := Table().Rows()
	if len(rows) != len(want) {
		t.Fatalf("%d rows against %d identifiers held here", len(rows), len(want))
	}

	for _, s := range rows {
		at, held := want[s.Name]
		if !held {
			t.Errorf("%s is new, so decide what Home Assistant calls it and hold it here", s.Name)
			continue
		}
		if got := s.Object(Table().Domain); got != at {
			t.Errorf("%s is %q, want %q", s.Name, got, at)
		}
	}
}

// Every row needs a name a person reads. say.T hands back the identifier for one nothing defines,
// so a missing key shows up in Home Assistant as "screen.setting.backlight".
func TestEveryRowIsNamed(t *testing.T) {
	for _, s := range Table().Rows() {
		if got := s.Named(); got == "" || strings.HasPrefix(got, "screen.") {
			t.Errorf("%s has no text: %q", s.Name, got)
		}
		for _, o := range s.Options {
			if got := s.Label(o); got == "" || strings.HasPrefix(got, "screen.") {
				t.Errorf("%s option %q has no text: %q", s.Name, o.Value, got)
			}
		}
	}
}

// A row writes the field it describes and nothing else, so two rows cannot fight over one value the
// way the camera's white balance did.
func TestARowWritesOnlyItsOwnField(t *testing.T) {
	for _, s := range Table().Rows() {
		was := config.Defaults().Screen
		moved := was

		other, ok := elsewhere(s.Name)
		if !ok {
			continue
		}
		if err := s.Write(&moved, other); err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}

		for _, at := range Table().Rows() {
			if at.Name == s.Name {
				continue
			}
			if before, after := at.Read(&was), at.Read(&moved); before != after {
				t.Errorf("writing %s moved %s from %q to %q", s.Name, at.Name, before, after)
			}
		}
	}
}

// elsewhere is a value each row takes that is not its default.
func elsewhere(name string) (string, bool) {
	switch name {
	case "backlight":
		return "13", true
	case "auto":
		return "off", true
	case "theme":
		return "Paper", true
	case "drawer":
		return string(config.EdgeTop), true
	}
	return "", false
}
