package livecam

import (
	"strings"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/camera/live"
)

func TestEveryCameraNameIsTranslated(t *testing.T) {
	for name, v := range map[string]live.Vendor{"qualcomm": qualcomm, "mediatek": mediatek} {
		table := live.New(live.Options{
			Vendor:  v,
			Streams: live.Streams{Main: qualcommMain, Sub: subWidths},
		}).Table()
		for _, g := range table.Groups() {
			if got := table.Title(g); strings.HasPrefix(got, "camera.") {
				t.Errorf("%s: the %s section has no text: %q", name, g, got)
			}
		}
		for _, s := range table.Rows() {
			if got := s.Title(); strings.HasPrefix(got, "camera.") {
				t.Errorf("%s: %s has no text: %q", name, s.Name, got)
			}
			for _, o := range s.Options {
				if got := s.Label(o); got == "" || strings.HasPrefix(got, "camera.") {
					t.Errorf("%s: %s option %q has no text: %q", name, s.Name, o.Value, got)
				}
			}
		}
	}
}
