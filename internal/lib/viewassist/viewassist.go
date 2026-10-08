// Written from View Assist's published documentation; none of its code (CC BY-NC 4.0) is here.
package viewassist

import (
	"slices"
	"strings"
)

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

var Views = []View{
	Alarm, Alert, Camera, Clock, Info, Intent, List, Music, Sports, Thermostat, Weather, Webpage,
}

func (v View) Drawable() bool { return v != Webpage && v.Known() }

func (v View) Known() bool { return slices.Contains(Views, v) }

const dashboard = "/viewassist/"

func (v View) Path() string { return dashboard + string(v) }

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

type Mode string

const (
	Normal       Mode = "normal"
	MusicMode    Mode = "music"
	Hold         Mode = "hold"
	Cycle        Mode = "cycle"
	Night        Mode = "night"
	DoNotDisturb Mode = "do-not-disturb"
)

func (m Mode) Idles() bool { return m != Hold && m != Cycle }

func (m Mode) Wakes() bool { return m != DoNotDisturb }
