// Package viewassist is View Assist's data contract, as types.
//
// View Assist drives a satellite by naming a view and handing it flat data — a title, a message, an
// image — rather than by sending markup. Their own browser route does not work on this device, so
// drawing the views natively is the only way it is a satellite at all. What that needs first is the
// contract written down in a form Go can check.
//
// The model only. It knows nothing about framebuffers or about ESPHome: LANovo wires actions in one
// side and internal/ui the other. It also keeps no timers — internal/feature/timer already runs
// those, and this describes the shape View Assist expects them in rather than running a second set.
//
// Written from the published documentation, which is a list of names and fields. None of View
// Assist's own code is here; it is CC BY-NC 4.0 and this tree is AGPLv3.
package viewassist

import (
	"slices"
	"strings"
)

// View is one of the screens View Assist can ask for.
type View string

const (
	Alarm      View = "alarm"
	Alert      View = "alert"
	Camera     View = "camera"
	Clock      View = "clock"
	Info       View = "info"
	Intent     View = "intent"
	List       View = "list"
	Music      View = "music"
	Sports     View = "sports"
	Thermostat View = "thermostat"
	Weather    View = "weather"
	Webpage    View = "webpage"
)

// Views is every view there is, alphabetical.
var Views = []View{
	Alarm, Alert, Camera, Clock, Info, Intent, List, Music, Sports, Thermostat, Weather, Webpage,
}

// Drawable reports whether this device can draw the view.
//
// All but one. A webpage is markup and there is no browser here, which is the whole reason for
// drawing the rest natively; asking for one is a request to decline rather than a failure.
func (v View) Drawable() bool { return v != Webpage && v.Known() }

// Known reports whether this is a view View Assist defines.
func (v View) Known() bool { return slices.Contains(Views, v) }

// dashboard is the path every view sits under by default. Configurable at their end, so a path is
// matched on its last segment rather than on the whole of it.
const dashboard = "/viewassist/"

// Path is the navigation path for a view, as their navigate action takes it.
func (v View) Path() string { return dashboard + string(v) }

// Navigated is the view a navigation path names.
//
// The last segment, because the dashboard the views sit under is configurable and a satellite that
// only recognised the default would stop understanding a renamed one. A path naming something that
// is not a view comes back not ok, which includes the external paths their status icons can carry.
func Navigated(path string) (View, bool) {
	at := strings.Trim(path, "/")
	if at == "" {
		return "", false
	}

	if cut := strings.LastIndex(at, "/"); cut >= 0 {
		at = at[cut+1:]
	}

	v := View(strings.ToLower(at))
	return v, v.Known()
}

// Mode is what the device is doing, which decides whether the idle timer runs and what it returns
// to.
//
// Documented as configurable per device rather than as a closed list, so an unknown one is carried
// rather than rejected: a mode this build has not heard of should leave the screen alone, not put
// it in the wrong state.
type Mode string

const (
	Normal       Mode = "normal"
	MusicMode    Mode = "music"
	Hold         Mode = "hold"
	Cycle        Mode = "cycle"
	Night        Mode = "night"
	DoNotDisturb Mode = "do-not-disturb"
)

// Idles reports whether the idle timer returns the screen to its default view in this mode.
//
// Hold is the mode that exists to stop it, and cycle is already moving through views of its own.
func (m Mode) Idles() bool { return m != Hold && m != Cycle }

// Wakes reports whether the device may put something on screen unasked.
//
// Do not disturb is the mode that says no. Night does not: a night mode is about how bright the
// screen is, and an alarm at night is the case that matters most.
func (m Mode) Wakes() bool { return m != DoNotDisturb }
