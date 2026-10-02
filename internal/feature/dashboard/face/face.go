// Package face is the ways the clock can be drawn.
//
// A face is handed the box it may use rather than the panel. Position, and a dashboard with more on
// it than the time, both give a face less than the whole screen, and a face that measured the screen
// itself would have to be taught about each of them.
//
// One file per face, registering itself from init. Nothing outside asks for a face by its type: the
// setting names it, and Of hands back the one that draws it. What a face needs that is not drawing
// lives apart from it — the reading in reading.go, the sizing in fit.go, the geometry beside the
// face that uses it — so a face file is the look of that face and nothing else.
//
// What a face does not decide is when it is drawn. The dashboard redraws when the reading changes,
// which is every minute, and a face that wanted a second hand would need a reading that carried
// seconds — worth doing when there is one, and dead weight until then.
package face

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Face draws a reading inside the box it is given, in the colors it is given.
//
// Inside the box is the contract: a face that paints outside it will overwrite whatever the
// dashboard put there, and the sweep test holds every face to it.
type Face interface {
	Draw(s ui.Surface, in ui.Rect, r Reading, palette theme.Theme)
}

// Ticking is a face that changes within the minute.
//
// The dashboard redraws when the reading changes, which is once a minute, and that is what keeps
// the panel idle. A face saying it ticks is asking for a repaint every second, so it is opt in and
// the cost lands only on the face that wanted it.
type Ticking interface {
	Ticks() bool
}

// Ticks reports whether the named face changes within the minute.
func Ticks(name config.Face) bool {
	t, ok := Of(name).(Ticking)
	return ok && t.Ticks()
}

// registered is every face there is, by the name the setting uses.
var registered = map[config.Face]Face{}

func register(name config.Face, f Face) { registered[name] = f }

// Of is the face a setting names, or the default for a name this build does not have — a device
// that came back from an upgrade without a clock would be worse than one showing the plain face.
func Of(name config.Face) Face {
	if f, ok := registered[name]; ok {
		return f
	}
	return registered[config.DefaultFace]
}
