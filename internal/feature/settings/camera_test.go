package settings

import (
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/setting"
	"github.com/ygelfand/LANovo/internal/ui/widget"
)

func TestTheCameraPageIsTheSections(t *testing.T) {
	rows, acts := cameraPage().Build()

	if len(rows) != len(shown())+2 || len(acts) != len(rows) {
		t.Fatalf(
			"%d rows against %d sections, the stream switch and a reset",
			len(rows),
			len(shown()),
		)
	}
	if stream := rows[len(rows)-2]; stream.Kind != widget.Toggle {
		t.Errorf("the row before reset is %+v, want the camera stream switch", stream)
	}
	if last := rows[len(rows)-1]; last.Kind == widget.Chevron || acts[len(acts)-1] == nil {
		t.Errorf("the last row is %+v, want the reset action", last)
	}
	for i, g := range shown() {
		if rows[i].Kind != widget.Chevron || acts[i] == nil {
			t.Errorf("%s does not open", g)
		}
	}
}

func TestASectionIsTheKnobTable(t *testing.T) {
	var seen int
	for _, g := range shown() {
		rows, acts := sectionPage(g).Build()
		want := livecam.Table().In(g)
		if len(rows) != len(want) || len(rows) != len(acts) {
			t.Fatalf("%s: %d rows, %d actions, %d knobs", g, len(rows), len(acts), len(want))
		}
		seen += len(rows)
		for i, s := range want {
			if rows[i].Label != s.Title() {
				t.Errorf("%s row %d is %q and the knob is %q", g, i, rows[i].Label, s.Title())
			}
			switch s.Kind {
			case setting.Number:
				if rows[i].Kind != widget.Slider || rows[i].Level < 0 || rows[i].Level > 100 {
					t.Errorf("%s drew as %v at %d", s.Name, rows[i].Kind, rows[i].Level)
				}
			case setting.Choice:
				if rows[i].Kind != widget.Chevron {
					t.Errorf("%s is a choice and drew as %v", s.Name, rows[i].Kind)
				}
			}
		}
	}
	if seen != len(livecam.Table().Rows()) {
		t.Errorf(
			"the sections show %d knobs and the table has %d",
			seen,
			len(livecam.Table().Rows()),
		)
	}
}

func TestAChoiceOpensItsOptions(t *testing.T) {
	s, ok := livecam.Table().Find("whitebalance")
	if !ok {
		t.Fatal("no white balance knob")
	}
	rows, acts := choicePage(s).Build()
	if len(rows) != len(s.Options) {
		t.Fatalf("%d rows against %d options", len(rows), len(s.Options))
	}
	var marked int
	for i, o := range s.Options {
		if rows[i].Label != s.Label(o) {
			t.Errorf("row %d is %q, want %q", i, rows[i].Label, s.Label(o))
		}
		if rows[i].Chosen {
			marked++
		}
		if acts[i] == nil {
			t.Errorf("%q cannot be picked", s.Label(o))
		}
	}
	if marked != 1 {
		t.Errorf("%d options are marked as in effect, want exactly the one that is", marked)
	}
}

func TestEveryCameraNameIsTranslated(t *testing.T) {
	for _, g := range livecam.Table().Groups() {
		if got := livecam.Table().Title(g); strings.HasPrefix(got, "camera.") {
			t.Errorf("the %s section has no text: %q", g, got)
		}
	}
	for _, s := range livecam.Table().Rows() {
		if got := s.Title(); strings.HasPrefix(got, "camera.") {
			t.Errorf("%s has no text: %q", s.Name, got)
		}
		for _, o := range s.Options {
			if got := s.Label(o); got == "" || strings.HasPrefix(got, "camera.") {
				t.Errorf("%s option %q has no text: %q", s.Name, o.Value, got)
			}
		}
	}
}
