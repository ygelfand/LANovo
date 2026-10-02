package control

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

// byHand is every setting the harness deliberately cannot reach, and why. A value here is a
// decision; a value in neither this nor the settings table is an oversight, which is the whole
// point of the test below.
// manyRows is the fields more than one row may change, which is the fields that are a map rather
// than a value. Every camera setting lives under the same key in the config and has its own row,
// generated from the camera's own table, so they all report the same field and none of them is a
// duplicate of another.
//
// Anything not here still may not be written twice: two rows on one value is the mistake this
// catches, and it has caught it.
var manyRows = map[string]bool{
	"Camera.Settings": true,
	"Home.Control":    true,
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

	"Access.Control":  "the socket the harness is speaking over: a command that turned it off would cut the branch it is sitting on, and turning it on is meaningless from a connection that only exists when it already is. Home Assistant's switch, or `lanovod tools control`",
	"Network.Address": "read back from the lease, not chosen",

	// Nothing about these is deliberate. They are settings with no way to reach them from the
	// harness, listed so that adding another is a choice rather than an accident, and so this list
	// is the thing to work through rather than the config being re-read to find them.
	"Screen.Drawer":       "not wired yet",
	"Screen.Volume":       "not wired yet",
	"Feedback.Chime":      "not wired yet",
	"Diag.Interval":       "not wired yet",
	"Time.Home":           "not wired yet",
	"Time.Chosen":         "not wired yet",
	"Network.Verify":      "not wired yet",
	"Cast.Oracle":         "Home Assistant's text entity",
	"Cast.YouTube.Device": "made on first use, never chosen",
	"Cast.YouTube.Music":  "issued by YouTube on first use",
	"Cast.YouTube.Video":  "issued by YouTube on first use",
}

// leaves is every setting in the config, as a dotted path. A struct that holds other settings is
// not one itself, so the walk stops at anything that is not another part of the config.
func leaves(t reflect.Type, at string) []string {
	var out []string
	for i := range t.NumField() {
		f := t.Field(i)
		name := at + f.Name

		if f.Type.Kind() == reflect.Struct && f.Type.PkgPath() == t.PkgPath() {
			out = append(out, leaves(f.Type, name+".")...)
			continue
		}
		out = append(out, name)
	}
	return out
}

// Every setting is either reachable from the harness or written down as not being. A new one is
// neither until somebody decides, and this is what makes them decide.
//
// The failure it exists for: a setting added to the config and to Home Assistant, and to nothing
// else. It works, it is saved, and the only way to change it on the device is to find the row and
// tap it — which is the thing the harness exists to avoid, and which nothing reports.
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

// A setting named in the table has to exist. Renaming a config field otherwise leaves the harness
// claiming to change something that is not there, which only shows up as a command that reads back
// the wrong thing.
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

// How much of the device the harness can arrange, which is worth knowing rather than guessing at.
func TestHowMuchIsReachable(t *testing.T) {
	all := leaves(reflect.TypeOf(config.Config{}), "")
	t.Logf("%d settings, %d reachable from the harness, %d accounted for by hand",
		len(all), len(settings()), len(byHand))
}
