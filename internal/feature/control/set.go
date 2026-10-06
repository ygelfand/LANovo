package control

import (
	"fmt"
	"github.com/ygelfand/LANovo/internal/feature/weather"
	"slices"
	"strconv"
	"strings"

	"github.com/ygelfand/LANovo/internal/feature/api"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	panel "github.com/ygelfand/LANovo/internal/feature/settings"
	text "github.com/ygelfand/LANovo/internal/lib/say"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/a2dp"
	"github.com/ygelfand/LANovo/internal/feature/access"
	"github.com/ygelfand/LANovo/internal/feature/bluetooth"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/chromecast"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/idle"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/microphone"
	"github.com/ygelfand/LANovo/internal/feature/poster"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/rtspd"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/feature/sendspin"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	knob "github.com/ygelfand/LANovo/internal/setting"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/LANovo/internal/ui/visual"
	"github.com/ygelfand/libcountertop/pkg/display/style"
)

// Changing a setting from here rather than by tapping it.
//
// Every setting on the device can be reached with a finger, and until now that was the only way the
// harness could reach one: find the row, work out where it landed, tap it. That breaks the moment a
// page gains a row — twice now a sweep has silently moved a brightness slider or turned the date off
// because the row it meant to hit had shifted down. A tap is the right way to test the touch
// handling and the wrong way to arrange the device before testing something else.
//
// It goes through the same setters the screen and Home Assistant use, not through the config store,
// so whatever changes here is saved, redrawn and published exactly as if somebody had tapped it.
// A harness that wrote the file directly would be testing a path nothing else takes.

// setting is one thing that can be read and changed by name.
type setting struct {
	name string

	// field is the config it reads and writes, as a dotted path from Config. It is here so a test
	// can walk the config and say which settings nothing reaches: a value that exists in the file
	// and in Home Assistant but nowhere a harness can touch is half wired, and nothing else
	// notices.
	field string

	// says is the value as it stands, for reading one back or listing them all.
	says func(config.Config) string

	// use changes it, reporting what was wrong with the value rather than ignoring it: a harness
	// that silently did nothing is worse than no harness, because the test still runs.
	use func(string) error
}

func settings() []setting {
	clock, display := dashboard.Get(), screen.Get()

	return append([]setting{
		{"media.duck", "Media.DuckDB", func(c config.Config) string { return strconv.FormatFloat(c.Media.DuckDB, 'f', -1, 64) }, func(s string) error {
			db, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return fmt.Errorf("ducking depth: %w", err)
			}
			return volume.Get().SetDuckDB(db)
		}},
		{"clock.face", "Clock.Face", func(c config.Config) string { return string(c.Clock.Face) },
			choose(config.Faces(), clock.SetFace)},

		{"idle.after", "Idle.After", func(c config.Config) string { return string(c.Idle.After) },
			choose(config.Delays(), idle.Get().SetAfter)},

		{"idle.media", "Idle.Media", func(c config.Config) string { return string(c.Idle.Media) },
			choose(config.MediaDelays(), media.Get().SetIdle)},

		{"idle.face", "Idle.Face", func(c config.Config) string { return string(c.Idle.Face) },
			choose(config.IdleFaces(), idle.Get().SetFace)},

		{"idle.position", "Idle.Position", func(c config.Config) string { return string(c.Idle.Position) },
			choose(config.Positions(), idle.Get().SetPosition)},

		{"idle.align", "Idle.Align", func(c config.Config) string { return string(c.Idle.Align) },
			choose(config.Aligns(), idle.Get().SetAlign)},

		{"idle.size", "Idle.Size", func(c config.Config) string { return string(c.Idle.Size) },
			choose(config.Sizes(), idle.Get().SetSize)},

		{"idle.visual1", "Idle.First.Kind", func(c config.Config) string { return orNone(c.Idle.First.Kind) },
			idleKind(0)},

		{"idle.visual1.source", "Idle.First.Source", func(c config.Config) string { return string(c.Idle.First.Source) },
			choose(config.Sources(), func(s config.Source) { idle.Get().SetSource(0, s) })},

		{"idle.visual2", "Idle.Second.Kind", func(c config.Config) string { return orNone(c.Idle.Second.Kind) },
			idleKind(1)},

		{"idle.visual2.source", "Idle.Second.Source", func(c config.Config) string { return string(c.Idle.Second.Source) },
			choose(config.Sources(), func(s config.Source) { idle.Get().SetSource(1, s) })},

		{"clock.position", "Clock.Position", func(c config.Config) string { return string(c.Clock.Position) },
			choose(config.Positions(), clock.SetPosition)},

		{"clock.size", "Clock.Size", func(c config.Config) string { return string(c.Clock.Size) },
			choose(config.Sizes(), clock.SetSize)},

		{"clock.color", "Clock.Ink", func(c config.Config) string { return string(c.Clock.Ink) },
			choose(config.Inks(), clock.SetInk)},

		{"clock.date", "Clock.Date", func(c config.Config) string { return knob.OnOff(c.Clock.Date) },
			toggle(clock.SetDate)},

		{"clock.hours", "Screen.Hours", func(c config.Config) string { return string(c.Screen.Hours) },
			choose(config.HourFormats(), clock.SetHours)},

		{"screen.keyboard", "Screen.Keyboard", func(c config.Config) string { return string(c.Screen.Keyboard) },
			choose(config.KeyboardSizes(), panel.SetKeyboard)},

		{"screen.theme", "Screen.Theme", func(c config.Config) string { return c.Screen.Theme }, paint},

		{"screen.style", "Screen.Style", func(c config.Config) string { return screen.Table().Row("style").Read(&c.Screen) },
			func(v string) error { return screen.Get().Set("style", v) }},

		{"screen.size", "Screen.Size", func(c config.Config) string { return screen.Table().Row("size").Read(&c.Screen) },
			func(v string) error { return screen.Get().Set("size", v) }},

		{"screen.visual", "Visual.Kind", func(c config.Config) string { return c.Visual.Kind },
			choose(visual.Built(), visuals.Get().SetKind)},

		{"screen.visual.fps", "Visual.MaxFPS", func(c config.Config) string { return strconv.Itoa(c.Visual.MaxFPS) },
			fpsStep},

		{"screen.visual.seed", "Visual.Seed", func(c config.Config) string { return strconv.Itoa(c.Visual.Seed) },
			number(0, visual.SeedMost, visuals.Get().SetSeed)},

		{"screen.visual.label", "Visual.Label", func(c config.Config) string { return orNone(c.Visual.Label) },
			words(visuals.Get().SetLabel)},

		{"weather.entity", "Weather.Entity", func(c config.Config) string { return orNone(c.Weather.Entity) },
			words(weather.SetEntity)},

		{"weather.look", "Weather.Look", func(c config.Config) string { return string(c.Weather.Look) },
			choose(config.WeatherLooks(), weather.SetLook)},

		{"weather.dashboard", "Weather.Dashboard", func(c config.Config) string { return knob.OnOff(c.Weather.Dashboard) },
			toggle(weather.SetDashboard)},

		{"weather.idle", "Weather.Idle", func(c config.Config) string { return knob.OnOff(c.Weather.Idle) },
			toggle(weather.SetIdle)},

		{"weather.animate", "Weather.Animate", func(c config.Config) string { return knob.OnOff(c.Weather.Animate) },
			toggle(weather.SetAnimate)},

		{"weather.themed", "Weather.Themed", func(c config.Config) string { return knob.OnOff(c.Weather.Themed) },
			toggle(weather.SetThemed)},

		{"weather.position", "Weather.Position", func(c config.Config) string { return string(c.Weather.Position) },
			choose(config.Positions(), weather.SetPosition)},

		{"weather.align", "Weather.Align", func(c config.Config) string { return string(c.Weather.Align) },
			choose(config.Aligns(), weather.SetAlign)},

		{"weather.size", "Weather.Size", func(c config.Config) string { return string(c.Weather.Size) },
			choose(config.Sizes(), weather.SetSize)},

		{"screen.language", "Screen.Language", func(config.Config) string { return text.Chosen() },
			language},

		{"poster.on", "Poster.Enabled", func(c config.Config) string { return knob.OnOff(c.Poster.Enabled) },
			toggle(poster.Get().SetEnabled)},

		{"poster.every", "Poster.Every", func(c config.Config) string { return string(c.Poster.Every) },
			choose(config.PosterEveries(), poster.Get().SetEvery)},

		{"poster.server", "Poster.Server", func(c config.Config) string { return orNone(c.Poster.Server) },
			words(poster.Get().SetServer)},

		{"poster.key", "Poster.Key", func(c config.Config) string { return secret(c.Poster.Key) },
			words(poster.Get().SetKey)},

		{"poster.albums", "Poster.Albums", func(c config.Config) string { return orNone(c.Poster.Albums) },
			words(poster.Get().SetAlbums)},

		{"poster.tags", "Poster.Tags", func(c config.Config) string { return orNone(c.Poster.Tags) },
			words(poster.Get().SetTags)},

		{"screen.marks", "Screen.Marks", func(c config.Config) string { return knob.OnOff(c.Screen.Marks) },
			toggle(privacy.Get().SetMarks)},

		{"screen.logo", "Screen.Logo", func(c config.Config) string { return knob.OnOff(c.Screen.Logo) },
			toggle(clock.SetLogo)},

		{"screen.backlight", "Screen.Backlight", func(c config.Config) string { return strconv.Itoa(c.Screen.Backlight) },
			number(0, 100, display.SetBacklight)},

		{"screen.mode", "Screen.Mode", func(c config.Config) string { return string(c.Screen.Mode) },
			choose(config.ScreenModes(), display.SetMode)},

		{"cast.youtube.skip", "Cast.YouTube.Skip", func(c config.Config) string { return orNone(strings.Join(c.Cast.YouTube.Skip, ",")) },
			words(chromecast.Get().SetSkip)},

		{"cast.prime.persist", "Cast.Prime.Persist", func(c config.Config) string { return knob.OnOff(c.Cast.Prime.Persist) },
			toggle(chromecast.Get().SetPrimePersist)},

		{"cast.prime.skipintro", "Cast.Prime.SkipIntro", func(c config.Config) string { return knob.OnOff(c.Cast.Prime.SkipIntro) },
			toggle(chromecast.Get().SetPrimeSkipIntro)},

		{"cast.youtube.livedelay", "Cast.YouTube.LiveDelay", func(c config.Config) string { return strconv.Itoa(c.Cast.YouTube.LiveDelay) + " s" },
			number(config.LiveDelayLeast, config.LiveDelayMost, chromecast.Get().SetLiveDelay)},

		{"cast.youtube.lounge", "Cast.YouTube.OnDemand", func(c config.Config) string {
			if c.Cast.YouTube.OnDemand {
				return "demand"
			}
			return "always"
		}, func(s string) error {
			switch s {
			case "always":
				chromecast.Get().SetLoungeOnDemand(false)
				return nil
			case "demand":
				chromecast.Get().SetLoungeOnDemand(true)
				return nil
			}
			return fmt.Errorf("want always or demand")
		}},
		{"microphone.gain", "Microphone.Gain", func(c config.Config) string {
			return strconv.Itoa(c.Microphone.Gain-config.DefaultMicGain) + " dB"
		}, number(0, microphone.GainMost, microphone.Get().SetGain)},
		{"microphone.leveling", "Microphone.Leveling", func(c config.Config) string { return knob.OnOff(c.Microphone.Leveling) },
			toggle(microphone.Get().SetLeveling)},
		{"microphone.denoise", "Microphone.Denoise", func(c config.Config) string { return knob.OnOff(c.Microphone.Denoise) },
			toggle(microphone.Get().SetDenoising)},
		{"microphone.visualizer_lift", "Microphone.VisualizerLift", func(c config.Config) string {
			return strconv.Itoa(c.Microphone.VisualizerLift) + " dB"
		}, number(0, visuals.LiftMax, microphone.Get().SetLift)},
		{"microphone.sensitivity", "Microphone.Sensitivity", func(c config.Config) string {
			return strconv.Itoa(c.Microphone.Sensitivity) + " dB"
		}, number(4, 20, microphone.Get().SetSensitivity)},
		{"features.sendspin", "Sendspin.Enabled", func(c config.Config) string { return knob.OnOff(c.Sendspin.Enabled) },
			toggle(sendspin.Get().SetEnabled)},
		{"call.incoming", "Call.Incoming", func(c config.Config) string { return knob.OnOff(c.Call.Incoming) },
			toggle(call.SetIncoming)},
		{"call.auto_answer", "Call.AutoAnswer", func(c config.Config) string { return knob.OnOff(c.Call.AutoAnswer) },
			toggle(call.SetAutoAnswer)},
		{"call.pause_wake", "Call.PauseWake", func(c config.Config) string { return knob.OnOff(c.Call.PauseWake) },
			toggle(call.SetPauseWake)},
		{"call.auto_video", "Call.AutoVideo", func(c config.Config) string { return knob.OnOff(c.Call.AutoVideo) },
			toggle(call.SetAutoVideo)},
		{"call.stream", "Call.Stream", func(c config.Config) string { return string(c.Call.Stream) },
			choose(config.CallStreams(), call.SetStream)},

		{"features.bluetooth", "Bluetooth.Proxy", func(c config.Config) string { return knob.OnOff(c.Bluetooth.Proxy) },
			toggle(bluetooth.Get().SetProxy)},
		{"features.speaker", "Bluetooth.Speaker", func(c config.Config) string { return knob.OnOff(c.Bluetooth.Speaker) },
			toggle(a2dp.Get().SetEnabled)},

		{"features.cast", "Cast.Receiver", func(c config.Config) string { return knob.OnOff(c.Cast.Receiver) },
			toggle(chromecast.Get().SetReceiver)},

		{"api.adopted", "API.Adopted", func(c config.Config) string { return knob.OnOff(c.API.Adopted) },
			toggle(api.Get().SetAdopted)},

		{"features.rtsp", "RTSP.Enabled", func(c config.Config) string { return knob.OnOff(c.RTSP.Enabled) },
			toggle(rtspd.Get().SetEnabled)},

		// Over the cable is where this one is worth having: plug in, turn it on, unplug.
		{"access.adb", "Access.ADB", func(c config.Config) string { return knob.OnOff(c.Access.ADB) },
			toggle(access.Get().SetADB)},
	}, append(append(cameraRows(), presenceRows()...), homeRows()...)...)
}

func presenceRows() []setting {
	rows := make([]setting, 0, len(sensors.Table().Rows()))
	for _, s := range sensors.Table().Rows() {
		field := "Presence.Wake"
		if s.Name == "range" {
			field = "Presence.Range"
		}
		rows = append(rows, setting{
			name:  "presence." + s.Name,
			field: field,
			says:  func(c config.Config) string { return s.Read(&c.Presence) },
			use:   func(v string) error { return sensors.SetPresence(s.Name, v) },
		})
	}
	return rows
}

func homeRows() []setting {
	var rows []setting
	for _, s := range homecontrol.Selections {
		rows = append(rows, setting{
			name:  "home.control." + s.Key,
			field: "Home.Control",
			says:  func(c config.Config) string { return knob.OnOff(c.Home.Control[s.Key]) },
			use:   toggle(s.SetControlled),
		}, setting{
			name:  "home.group." + s.Key,
			field: "Home.Group",
			says:  func(c config.Config) string { return strconv.Itoa(c.Home.Grouped(s.Key)) },
			use:   number(0, config.HomeGroupMost, s.SetGroupFrom),
		})
	}
	rows = append(rows, setting{
		name:  "home.enabled",
		field: "Home.Enabled",
		says:  func(c config.Config) string { return knob.OnOff(c.Home.Enabled) },
		use:   toggle(homecontrol.SetEnabled),
	})
	rows = append(rows, setting{
		name:  "home.combine",
		field: "Home.Combine",
		says:  func(c config.Config) string { return knob.OnOff(c.Home.Combine) },
		use:   toggle(homecontrol.SetCombined),
	})
	return rows
}

func cameraRows() []setting {
	rows := make([]setting, 0, len(livecam.Table().Rows()))
	for _, s := range livecam.Table().Rows() {
		rows = append(rows, setting{
			name:  "camera." + s.Name,
			field: "Camera.Settings",
			says: func(c config.Config) string {
				k, _ := livecam.Table().Configured(livecam.DefaultKnobs(), c.Camera.Settings)
				return s.Read(&k)
			},
			use: func(v string) error {
				k := livecam.DefaultKnobs()
				if err := s.Put(&k, v); err != nil {
					return err
				}
				return livecam.Set(s.Name, v)
			},
		})
	}
	return rows
}

// set reads or changes a setting.
//
// With nothing it lists them, which is how somebody at a terminal finds the name without reading
// the source, and is also the quickest way to see everything the device is set to at once.
func set(args []string) (string, error) {
	all := settings()

	if len(args) == 0 {
		cfg := config.Get()

		var out []string
		for _, s := range all {
			out = append(out, fmt.Sprintf("%-18s %s", s.name, s.says(cfg)))
		}
		return strings.Join(out, "\n"), nil
	}

	at := -1
	for i, s := range all {
		if strings.EqualFold(s.name, args[0]) {
			at = i
			break
		}
	}
	if at < 0 {
		return "", fmt.Errorf("no such setting %q, try set with no arguments", args[0])
	}

	if len(args) == 1 {
		return all[at].says(config.Get()), nil
	}

	// The rest joined, so a value with a space in it — a theme called Deep Ocean — arrives whole
	// rather than as the first word and a complaint.
	if err := all[at].use(strings.Join(args[1:], " ")); err != nil {
		return "", fmt.Errorf("%s: %w", all[at].name, err)
	}
	return all[at].says(config.Get()), nil
}

// choose matches one of a labeled setting's values.
//
// By label or by the name in the file, either way and ignoring case. Both because they are not the
// same string and each is what somebody would reasonably type: the screen says "Analog seconds" and
// the file says "analog-seconds".
// language picks the text the panel shows, by tag or by the name a language calls itself.
func language(s string) error {
	for _, tag := range text.Languages() {
		if strings.EqualFold(tag, s) || strings.EqualFold(text.Name(tag), s) {
			panel.SetLanguage(tag)
			return nil
		}
	}

	have := make([]string, 0, len(text.Languages()))
	for _, tag := range text.Languages() {
		have = append(have, tag)
	}
	return fmt.Errorf("want one of %s", strings.Join(have, ", "))
}

func idleKind(slot int) func(string) error {
	return func(s string) error {
		if strings.EqualFold(s, "none") || s == "" {
			idle.Get().SetKind(slot, "")
			return nil
		}
		for _, k := range visual.Built() {
			if strings.EqualFold(string(k), s) || strings.EqualFold(k.Label(), s) {
				idle.Get().SetKind(slot, string(k))
				return nil
			}
		}
		return fmt.Errorf("want none or one of %s", strings.Join(config.Labels(visual.Built()), ", "))
	}
}

func choose[T config.Labeled](values []T, use func(T)) func(string) error {
	return func(s string) error {
		for _, v := range values {
			if strings.EqualFold(v.Label(), s) || strings.EqualFold(fmt.Sprint(v), s) {
				use(v)
				return nil
			}
		}
		return fmt.Errorf("want one of %s", strings.Join(config.Labels(values), ", "))
	}
}

func words(use func(string)) func(string) error {
	return func(s string) error {
		use(s)
		return nil
	}
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

func secret(s string) string {
	if s == "" {
		return "(none)"
	}
	return "(set)"
}

// toggle reads the words people actually type for a switch.
func toggle(use func(bool)) func(string) error {
	return func(s string) error {
		on, ok := knob.Boolean(s)
		if !ok {
			return fmt.Errorf("want on or off")
		}
		use(on)
		return nil
	}
}

// number reads a level, refusing one outside the range rather than quietly clamping it: a harness
// that asked for 150 and got 100 is a test that passed for the wrong reason.
func number(lo, hi int, use func(int)) func(string) error {
	return func(s string) error {
		v, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("want a number from %d to %d", lo, hi)
		}
		if v < lo || v > hi {
			return fmt.Errorf("want %d to %d, not %d", lo, hi, v)
		}
		use(v)
		return nil
	}
}

func fpsStep(s string) error {
	v, err := strconv.Atoi(s)
	if err != nil || !slices.Contains(config.MaxFPSSteps, v) {
		return fmt.Errorf("want one of %v", config.MaxFPSSteps)
	}
	visuals.Get().SetMaxFPS(v)
	return nil
}

// paint changes the theme, which is named rather than chosen from a labeled set.
func paint(s string) error {
	if strings.EqualFold(s, style.ThemeDefault) {
		if err := config.Set().Screen().Theme(style.ThemeDefault); err != nil {
			return err
		}
		screen.Get().Use(style.ThemeDefault)
		return nil
	}
	for _, t := range theme.All {
		if !strings.EqualFold(t.Name, s) {
			continue
		}

		if err := config.Set().Screen().Theme(t.Name); err != nil {
			return err
		}
		screen.Get().Use(t.Name)
		return nil
	}

	var names []string
	for _, t := range theme.All {
		names = append(names, t.Name)
	}
	return fmt.Errorf("want one of %s", strings.Join(names, ", "))
}
