package control

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

var manyRows = map[string][]string{
	"Camera.Settings":      {"camera."},
	"Doorbell.Doors":       {"doorbell.door"},
	"Photos.Google.Albums": {"photos.google.album"},
	"Home.Control":         {"home.control."},
	"Home.Group":           {"home.group."},
	"Screen.Custom":        {"screen.custom."},
	"Voice.Off":            {"voice.action.", "voice.command."},
}

var inTables = map[string]string{
	"Feedback.Chime":            "feedback.chime",
	"Feedback.Failure":          "feedback.failure",
	"Feedback.Cancel":           "feedback.cancel",
	"Feedback.Mute":             "feedback.mute",
	"Feedback.Unmute":           "feedback.unmute",
	"Alerts.Info":               "alerts.info",
	"Alerts.Success":            "alerts.success",
	"Alerts.Warning":            "alerts.warning",
	"Alerts.Alert":              "alerts.alert",
	"Alerts.Timer":              "alerts.timer",
	"Alerts.RingFor":            "alerts.ring_for",
	"Alerts.Wake":               "alerts.wake",
	"Media.DuckDB":              "media.duck",
	"Media.Buttons":             "media.buttons",
	"Poster.Enabled":            "poster.on",
	"Poster.Every":              "poster.every",
	"Poster.Fit":                "poster.fit",
	"Poster.Change":             "poster.transition",
	"Photos.AppChange":          "photos.app.transition",
	"Idle.PhotoChange":          "idle.photo_transition",
	"Photos.AppFit":             "photos.app.fit",
	"Photos.Immich.Quality":     "photos.immich.quality",
	"Idle.PhotoFit":             "idle.photo_fit",
	"Photos.Google.Enabled":     "photos.google.enabled",
	"Photos.AppEvery":           "photos.app.every",
	"Photos.Immich.Enabled":     "photos.immich.enabled",
	"Photos.Immich.Server":      "photos.immich.server",
	"Photos.Immich.Key":         "photos.immich.key",
	"Photos.Immich.Albums":      "photos.immich.albums",
	"Photos.Immich.Tags":        "photos.immich.tags",
	"Photos.Immich.People":      "photos.immich.people",
	"Photos.Immich.Favorites":   "photos.immich.favorites",
	"Photos.Immich.Memories":    "photos.immich.memories",
	"Cast.Receiver":             "cast.receiver",
	"Cast.YouTube.Skip":         "cast.youtube.skip",
	"Sendspin.Enabled":          "media.sendspin",
	"AirPlay.Enabled":           "media.airplay",
	"AirPlay.Audio":             "media.airplay_audio_mode",
	"Screen.Backlight":          "screen.backlight",
	"Screen.Mode":               "screen.auto",
	"Screen.Theme":              "screen.theme",
	"Screen.Style":              "screen.style",
	"Screen.Size":               "screen.size",
	"Screen.Drawer":             "screen.drawer",
	"Screen.DockHide":           "screen.drawer.hide",
	"Apps.Order":                "apps.order",
	"Apps.Dock":                 "apps.dock",
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
	"Idle.PhotoEvery":           "idle.photo_every",
	"Idle.Night":                "idle.night",
	"Idle.NightLux":             "idle.night.lux",
	"Idle.NightAfter":           "idle.night.after",
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
	"Cameras.Tab":               "cameras.tab",
	"Certs.Server":              "certs.server",
	"Certs.Cert":                "certs.cert",
	"Certs.CertAPIKey":          "certs.cert_api_key",
	"Certs.KeyAPIKey":           "certs.key_api_key",
	"Frontend.Password":         "access.password",
	"Cameras.Scale":             "cameras.scale",
	"Cameras.Refresh":           "cameras.refresh",
	"Cameras.Rotate":            "cameras.rotate",
	"Cameras.URL":               "cameras.url",
	"Doorbell.Chime":            "doorbell.chime",
	"Doorbell.Notify":           "doorbell.notify",
	"Doorbell.RingFor":          "doorbell.ring_for",
	"Doorbell.LiveFor":          "doorbell.live_for",
	"Doorbell.Style":            "doorbell.style",
	"Doorbell.Visitor":          "doorbell.visitor",
	"Doorbell.Tab":              "doorbell.tab",
	"HomeAlarm.Entity":          "homealarm.entity",
	"HomeAlarm.Look":            "homealarm.look",
	"HomeAlarm.Tab":             "homealarm.tab",
	"HomeAlarm.Triggered.Show":  "homealarm.triggered.show",
	"HomeAlarm.Triggered.Sound": "homealarm.triggered.sound",
	"HomeAlarm.Pending.Show":    "homealarm.pending.show",
	"HomeAlarm.Pending.Sound":   "homealarm.pending.sound",
	"HomeAlarm.Arming.Show":     "homealarm.arming.show",
	"HomeAlarm.Arming.Sound":    "homealarm.arming.sound",
	"HomeAlarm.Settled.Show":    "homealarm.settled.show",
	"HomeAlarm.Settled.Sound":   "homealarm.settled.sound",
	"HomeAlarm.SettledFor":      "homealarm.settled.for",
	"Thermostat.Look":           "thermostat.look",
	"Thermostat.Tab":            "thermostat.tab",
	"Network.HomeKey":           "network.home_key",
	"Microphone.Gain":           "microphone.gain",
	"Microphone.Leveling":       "microphone.leveling",
	"Microphone.Denoise":        "microphone.denoise",
	"Microphone.Sensitivity":    "microphone.sensitivity",
	"Call.Incoming":             "call.incoming",
	"Call.Broadcasts":           "call.broadcasts",
	"Call.AutoAnswer":           "call.auto_answer",
	"Call.PauseWake":            "call.pause_wake",
	"Call.AutoVideo":            "call.auto_video",
	"Call.Stream":               "call.stream",
	"Call.AnswerFrom":           "call.answer_from",
	"Call.AnswerPeers":          "call.answer_peers",
	"Call.AnswerUsers":          "call.answer_users",
	"Call.AnswerAPI":            "call.answer_api",
	"Call.Ring":                 "call.ring",
	"Call.VideoRing":            "call.video_ring",
	"Call.Ringback":             "call.ringback",
	"Calendar.Entities":         "calendar.entities",
	"Calendar.Span":             "calendar.span",
	"Todo.Entity":               "todo.entity",
	"Todo.Completed":            "todo.completed",
	"Voice.Commands":            "voice.commands",
	"Voice.Actions":             "voice.actions",
	"Voice.Label":               "voice.label",
	"Weather.Alerts":            "weather.alerts",
	"Weather.RadarSource":       "weather.radar_source",
	"Weather.RadarRange":        "weather.radar_range",
	"Bluetooth.Proxy":           "bluetooth.proxy",
	"Bluetooth.Speaker":         "bluetooth.speaker",
	"Bluetooth.Scan":            "bluetooth.scan",
	"API.Adopted":               "access.adopted",
	"RTSP.Enabled":              "rtsp.enabled",
	"RTSP.Username":             "rtsp.username",
	"RTSP.Password":             "rtsp.password",
	"Lock.PIN":                  "screen.lock_pin",
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
	"Home.Themed":               "home.tessera.themed",
	"Home.Turn":                 "home.tessera.turn",
	"Home.Bottom":               "home.tessera.bottom",
	"Home.Follow":               "home.tessera.follow",
	"Diag.Interval":             "diag.interval",
	"Diag.LowMemory":            "diag.low_memory",
	"Diag.PageMemory":           "diag.page_memory",
	"Wake.Stop.Threshold":       "assistants.stop_sensitivity",
	"Wake.Pryon":                "assistants.pryon",
	"Wake.Stop.Pryon":           "assistants.pryon_stop",
	"Wake.Gemini":               "assistants.gemini",
}

var byHand = map[string]string{
	"Device.Name":  "told to the process at start-up, never written to the file",
	"Device.Addr":  "the same",
	"Device.Model": "the same, read from the hardware",

	"HomeAlarm.Hide": "the home alarm's zone list page, one toggle per discovered zone",

	"Thermostat.Entities": "the thermostat page's climate list, one toggle per entity, up to four",

	"Volume.Main":     "the volume command sets these, by stream",
	"Volume.Media":    "the volume command",
	"Volume.Alerts":   "the volume command",
	"Volume.Voice":    "the volume command",
	"Volume.Feedback": "the volume command",

	"Wake.Words": "Home Assistant's, by slot, through the satellite",

	"Poster.Last": "the picture last shown, written by the poster itself",

	"Lock.Failures": "wrong PINs counted by the lock screen",
	"Lock.Until":    "the lockout the lock screen sets after too many wrong PINs",

	"Radio.Favorites": "starred on the radio screen",
	"Radio.Last":      "the station last tuned, written by the radio itself",
	"Radio.LastTitle": "the same",

	"Home.Picks":            "Home control's entity pickers, keyed by selection",
	"Cameras.Pick.All":      "the cameras app's entity picker",
	"Cameras.Pick.Labels":   "the cameras app's entity picker",
	"Cameras.Pick.Areas":    "the cameras app's entity picker",
	"Cameras.Pick.Entities": "the cameras app's entity picker",

	"Update.Channel":     "Home Assistant's update channel select",
	"Update.LastVersion": "the version last announced to Home Assistant, written by the firmware feature itself",

	"Network.Address": "read back from the lease, not chosen",

	"Screen.Volume":         "not wired yet",
	"Feedback.Hold":         "the held mute button sound is only exposed by hardware with a hold action",
	"Diag.MinCores":         "held online only on boards whose kernel hotplugs cores",
	"Time.Home":             "not wired yet",
	"Time.Chosen":           "not wired yet",
	"Cast.Oracle":           "Home Assistant's text entity",
	"Cast.Bundle":           "Home Assistant's text entity",
	"Cast.YouTube.Device":   "the Echo Dot's YouTube lounge; LANovo runs none",
	"Cast.YouTube.Music":    "the Echo Dot's YouTube lounge; LANovo runs none",
	"Cast.YouTube.Video":    "the Echo Dot's YouTube lounge; LANovo runs none",
	"Cast.YouTube.OnDemand": "the Echo Dot's YouTube lounge; LANovo runs none",
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
	var prefixes []string
	for field, ps := range manyRows {
		reached[field] = ps[0]
		prefixes = append(prefixes, ps...)
	}
	for _, s := range settings() {
		if s.Field == "" {
			if !slices.Contains(slices.Collect(maps.Values(inTables)), s.Name) &&
				!slices.ContainsFunc(prefixes, func(p string) bool {
					return strings.HasPrefix(s.Name, p)
				}) {
				t.Errorf("%s does not say which setting it changes", s.Name)
			}
			continue
		}
		if was, twice := reached[s.Field]; twice && manyRows[s.Field] == nil {
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
