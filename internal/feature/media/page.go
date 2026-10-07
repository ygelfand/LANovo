package media

import (
	"sync"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/drawer"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/libcountertop/pkg/say"
)

// Page is what is playing, with the transport for it. One instance, so the shell can recognize it.
func Page() shell.View {
	pageOnce.Do(func() { page = &screen{} })
	return page
}

var (
	pageOnce sync.Once
	page     *screen
)

// Showing reports whether the player is the screen being looked at.
func Showing() bool { return shell.Get().Top() == Page() }

// onRail puts the player in the dock, so it can be reached when nothing is playing.
func onRail() {
	drawer.Get().Add(drawer.Entry{
		Name:  func() string { return say.T("rail.media") },
		Order: drawer.OrderPlayer,
		Glyph: func() string { return gogui.IconMusic },
		Open: func() {
			if now := Get().Now(); now.Playing || now.Paused {
				Get().Open()
				return
			}
			shell.Get().Push(Page())
		},
	})
}

type screen struct {
	mu    sync.Mutex
	shown time.Duration
}

func (v *screen) Covers() bool { return true }

func (v *screen) Shows(s config.Stream) bool { return s == config.StreamMedia }

func (v *screen) Timeout() time.Duration { return shell.SettingsTimeout }

func (v *screen) moved(elapsed time.Duration) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	second := elapsed / time.Second
	if second == v.shown {
		return false
	}
	v.shown = second
	return true
}

func Heading(now Now) string {
	switch {
	case now.Title != "":
		return now.Title
	case now.Paused:
		return "Paused"
	case now.Playing:
		return "Playing"
	}
	return "Nothing playing"
}

// transport is whoever the buttons reach: whoever holds the card, or Home Assistant before anything
// has played.
func Transport() Source {
	if s := Get().source(); s != nil {
		return s
	}
	return homeAssistant{}
}

// homeAssistant is a url played at this device through the media player entity: a peer of a group,
// a phone and a cast, claiming the card the same way.
//
// It answers only to stopping, since a url has no transport beyond ending it.
type homeAssistant struct{}

func (homeAssistant) Play()         {}
func (homeAssistant) Pause()        {}
func (homeAssistant) Next()         {}
func (homeAssistant) Previous()     {}
func (homeAssistant) Stop()         { Get().Stop() }
func (homeAssistant) Now() Now      { return Get().queued() }
func (homeAssistant) Kind() Kind    { return FromQueue }
func (homeAssistant) Label() string { return "" }
