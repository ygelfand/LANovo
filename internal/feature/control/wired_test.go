package control

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

var manyRows = map[string]bool{
	"Camera.Settings": true,
	"Home.Control":    true,
	"Home.Group":      true,
}

var byHand = map[string]string{
	"Device.Name":  "told to the process at start-up, never written to the file",
	"Device.Addr":  "the same",
	"Device.Model": "the same, read from the hardware",

	"Volume.Media":    "the volume command sets these, by stream",
	"Volume.Alerts":   "the volume command",
	"Volume.Voice":    "the volume command",
	"Volume.Feedback": "the volume command",

	"Wake.Words":          "Home Assistant's, by slot, through the satellite",
	"Wake.Stop.Threshold": "an entity on the microphone page; a harness that changed it would be turning detection off under whatever else it was testing",

	"API.Adopted": "written once when Home Assistant first subscribes",

	"Poster.Last": "the picture last shown, written by the poster itself",

	"Home.Picks": "Home control's entity pickers, keyed by selection",

	"Update.Channel":     "Home Assistant's update channel select",
	"Update.LastVersion": "the version last announced to Home Assistant, written by the firmware feature itself",

	"Network.Address": "read back from the lease, not chosen",

	"Screen.Drawer":       "not wired yet",
	"Screen.Volume":       "not wired yet",
	"Feedback.Chime":      "not wired yet",
	"Diag.Interval":       "not wired yet",
	"Time.Home":           "not wired yet",
	"Time.Chosen":         "not wired yet",
	"Cast.Oracle":         "Home Assistant's text entity",
	"Cast.YouTube.Device": "made on first use, never chosen",
	"Cast.YouTube.Music":  "issued by YouTube on first use",
	"Cast.YouTube.Video":  "issued by YouTube on first use",
}

func leaves(t reflect.Type, at string) []string {
	var out []string
	for i := range t.NumField() {
		f := t.Field(i)
		name := at + f.Name

		if f.Type.Kind() == reflect.Struct &&
			(f.Type.PkgPath() == t.PkgPath() || f.Type.PkgPath() == "github.com/ygelfand/libcountertop/pkg/settings/schema") {
			out = append(out, leaves(f.Type, name+".")...)
			continue
		}
		out = append(out, name)
	}
	return out
}

func TestEverySettingIsReachedOrAccountedFor(t *testing.T) {
	reached := map[string]string{}
	for _, s := range settings() {
		if s.field == "" {
			t.Errorf("%s does not say which setting it changes", s.name)
			continue
		}
		if was, twice := reached[s.field]; twice && !manyRows[s.field] {
			t.Errorf("%s is changed by both %s and %s", s.field, was, s.name)
		}
		reached[s.field] = s.name
	}

	var missing []string
	for _, field := range leaves(reflect.TypeOf(config.Config{}), "") {
		if _, ok := reached[field]; ok {
			continue
		}
		if _, ok := byHand[field]; ok {
			continue
		}
		missing = append(missing, field)
	}

	if len(missing) > 0 {
		t.Errorf("nothing reaches these settings, and byHand does not say why:\n\t%s",
			strings.Join(missing, "\n\t"))
	}
}

func TestEverySettingNamesARealField(t *testing.T) {
	all := leaves(reflect.TypeOf(config.Config{}), "")

	for _, s := range settings() {
		if s.field != "" && !slices.Contains(all, s.field) {
			t.Errorf("%s changes %s, which is not in the config", s.name, s.field)
		}
	}
	for field := range byHand {
		if !slices.Contains(all, field) {
			t.Errorf("byHand lists %s, which is not in the config", field)
		}
	}
}

func TestHowMuchIsReachable(t *testing.T) {
	all := leaves(reflect.TypeOf(config.Config{}), "")
	t.Logf("%d settings, %d reachable from the harness, %d accounted for by hand",
		len(all), len(settings()), len(byHand))
}
