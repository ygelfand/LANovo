package control

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

var manyRows = map[string]string{
	"Camera.Settings": "camera.",
	"Home.Control":    "home.control.",
	"Home.Group":      "home.group.",
}

var inTables = map[string]string{
	"Alerts.Info":               "alerts.info",
	"Alerts.Success":            "alerts.success",
	"Alerts.Warning":            "alerts.warning",
	"Alerts.Alert":              "alerts.alert",
	"Alerts.Timer":              "alerts.timer",
	"Alerts.RingFor":            "alerts.ring_for",
	"Media.DuckDB":              "media.duck",
	"Poster.Enabled":            "poster.on",
	"Poster.Every":              "poster.every",
	"Poster.Server":             "poster.server",
	"Poster.Key":                "poster.key",
	"Poster.Albums":             "poster.albums",
	"Poster.Tags":               "poster.tags",
	"Cast.Receiver":             "cast.receiver",
	"Cast.YouTube.Skip":         "cast.youtube.skip",
	"Cast.YouTube.LiveDelay":    "cast.youtube.livedelay",
	"Cast.YouTube.OnDemand":     "cast.youtube.lounge",
	"Cast.Prime.Persist":        "cast.prime.persist",
	"Cast.Prime.SkipIntro":      "cast.prime.skipintro",
	"Sendspin.Enabled":          "sendspin.on",
	"Screen.Backlight":          "screen.backlight",
	"Screen.Mode":               "screen.auto",
	"Screen.Theme":              "screen.theme",
	"Screen.Style":              "screen.style",
	"Screen.Size":               "screen.size",
	"Screen.Drawer":             "screen.drawer",
	"Screen.Alert":              "screen.alert",
	"Screen.Timer":              "screen.timer",
	"Screen.Keyboard":           "screen.keyboard",
	"Screen.Language":           "screen.language",
	"Screen.Marks":              "screen.marks",
	"Screen.Hours":              "clock.hours",
	"Clock.Face":                "clock.face",
	"Clock.Position":            "clock.position",
	"Clock.Align":               "clock.align",
	"Clock.Size":                "clock.size",
	"Clock.Ink":                 "clock.color",
	"Clock.Date":                "clock.date",
	"Idle.After":                "idle.after",
	"Idle.Media":                "idle.media",
	"Idle.Face":                 "idle.face",
	"Idle.Position":             "idle.position",
	"Idle.Align":                "idle.align",
	"Idle.Size":                 "idle.size",
	"Idle.First.Kind":           "idle.visual1",
	"Idle.First.Source":         "idle.visual1.source",
	"Idle.Second.Kind":          "idle.visual2",
	"Idle.Second.Source":        "idle.visual2.source",
	"Idle.Saver":                "idle.saver",
	"Visual.Kind":               "visual.kind",
	"Visual.MaxFPS":             "visual.fps",
	"Visual.Seed":               "visual.seed",
	"Visual.Label":              "visual.label",
	"Microphone.VisualizerLift": "visual.lift",
	"Weather.Entity":            "weather.entity",
	"Weather.Look":              "weather.look",
	"Weather.Dashboard":         "weather.dashboard",
	"Weather.Idle":              "weather.idle",
	"Weather.Animate":           "weather.animate",
	"Weather.Themed":            "weather.themed",
	"Weather.Position":          "weather.position",
	"Weather.Align":             "weather.align",
	"Weather.Size":              "weather.size",
	"Weather.Saver.Look":        "weather.saver.look",
	"Weather.Saver.Animate":     "weather.saver.animate",
	"Weather.Saver.Themed":      "weather.saver.themed",
	"Weather.Saver.Position":    "weather.saver.position",
	"Weather.Saver.Align":       "weather.saver.align",
	"Weather.Saver.Size":        "weather.saver.size",
	"Network.Verify":            "network.verify",
	"Microphone.Gain":           "microphone.gain",
	"Microphone.Leveling":       "microphone.leveling",
	"Microphone.Denoise":        "microphone.denoise",
	"Microphone.Sensitivity":    "microphone.sensitivity",
	"Call.Incoming":             "call.incoming",
	"Call.AutoAnswer":           "call.auto_answer",
	"Call.PauseWake":            "call.pause_wake",
	"Call.AutoVideo":            "call.auto_video",
	"Call.Stream":               "call.stream",
	"Bluetooth.Proxy":           "bluetooth.proxy",
	"Bluetooth.Speaker":         "bluetooth.speaker",
	"Bluetooth.Scan":            "bluetooth.scan",
	"API.Adopted":               "api.adopted",
	"RTSP.Enabled":              "rtsp.enabled",
	"Access.ADB":                "access.adb",
	"Presence.Wake":             "presence.wake",
	"Presence.Range":            "presence.range",
	"Home.Enabled":              "home.enabled",
	"Home.Combine":              "home.combine",
	"Home.Mode":                 "home.dashboard",
	"Home.Tessera.Columns":      "home.tessera.columns",
	"Home.Tessera.Rows":         "home.tessera.rows",
	"Home.Return":               "home.tessera.home_after_idle",
	"Home.Wide":                 "home.tessera.landscape",
	"Home.Tall":                 "home.tessera.portrait",
}

var byHand = map[string]string{
	"Device.Name":  "told to the process at start-up, never written to the file",
	"Device.Addr":  "the same",
	"Device.Model": "the same, read from the hardware",

	"Volume.Main":     "the volume command sets these, by stream",
	"Volume.Media":    "the volume command",
	"Volume.Alerts":   "the volume command",
	"Volume.Voice":    "the volume command",
	"Volume.Feedback": "the volume command",

	"Wake.Words":          "Home Assistant's, by slot, through the satellite",
	"Wake.Stop.Threshold": "an entity on the microphone page; a harness that changed it would be turning detection off under whatever else it was testing",

	"Poster.Last": "the picture last shown, written by the poster itself",

	"Home.Picks": "Home control's entity pickers, keyed by selection",

	"Update.Channel":     "Home Assistant's update channel select",
	"Update.LastVersion": "the version last announced to Home Assistant, written by the firmware feature itself",

	"Network.Address": "read back from the lease, not chosen",

	"Screen.Volume":       "not wired yet",
	"Feedback.Chime":      "not wired yet",
	"Diag.Interval":       "not wired yet",
	"Time.Home":           "not wired yet",
	"Time.Chosen":         "not wired yet",
	"Cast.Oracle":         "Home Assistant's text entity",
	"Cast.Bundle":         "Home Assistant's text entity",
	"Cast.YouTube.Device": "made on first use, never chosen",
	"Cast.YouTube.Music":  "issued by YouTube on first use",
	"Cast.YouTube.Video":  "issued by YouTube on first use",
}

func leaves(t reflect.Type, at string) []string {
	var out []string
	for _, f := range reflect.VisibleFields(t) {
		if f.Anonymous {
			continue
		}
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
	for field, name := range inTables {
		reached[field] = name
	}
	for field, prefix := range manyRows {
		reached[field] = prefix
	}
	for _, s := range settings() {
		if s.Field == "" {
			if !slices.Contains(slices.Collect(maps.Values(inTables)), s.Name) &&
				!slices.ContainsFunc(slices.Collect(maps.Values(manyRows)), func(p string) bool {
					return strings.HasPrefix(s.Name, p)
				}) {
				t.Errorf("%s does not say which setting it changes", s.Name)
			}
			continue
		}
		if was, twice := reached[s.Field]; twice && manyRows[s.Field] == "" {
			t.Errorf("%s is changed by both %s and %s", s.Field, was, s.Name)
		}
		reached[s.Field] = s.Name
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
		if s.Field != "" && !slices.Contains(all, s.Field) {
			t.Errorf("%s changes %s, which is not in the config", s.Name, s.Field)
		}
	}
	for field := range byHand {
		if !slices.Contains(all, field) {
			t.Errorf("byHand lists %s, which is not in the config", field)
		}
	}
	for field := range inTables {
		if !slices.Contains(all, field) {
			t.Errorf("inTables lists %s, which is not in the config", field)
		}
	}
}

func TestHowMuchIsReachable(t *testing.T) {
	all := leaves(reflect.TypeOf(config.Config{}), "")
	t.Logf("%d settings, %d reachable from the harness, %d accounted for by hand",
		len(all), len(settings()), len(byHand))
}
