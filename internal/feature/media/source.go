package media

import (
	"fmt"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/libcountertop/pkg/display/mark"
	sharedsource "github.com/ygelfand/libcountertop/pkg/media/source"
)

type Source = sharedsource.Source
type Details = sharedsource.Details
type Controller = sharedsource.Controller
type Now = sharedsource.Now
type Track = sharedsource.Track
type Kind = sharedsource.Kind
type Controls = sharedsource.Controls
type Seeker = sharedsource.Seeker
type Opener = sharedsource.Opener

const (
	FromQueue     = sharedsource.FromQueue
	FromBluetooth = sharedsource.FromBluetooth
	FromGroup     = sharedsource.FromGroup
	FromCast      = sharedsource.FromCast
	CanPause      = sharedsource.CanPause
	CanStop       = sharedsource.CanStop
	CanNext       = sharedsource.CanNext
	CanPrevious   = sharedsource.CanPrevious
)

func KindIcon(k Kind) ui.Icon {
	switch k {
	case FromBluetooth:
		return mark.Bluetooth
	case FromGroup:
		return mark.Speakers
	case FromCast:
		return mark.Chromecast
	}
	return mark.HomeAssistant
}

// From is the kind holding the card, which is the queue when nothing else has it.
func (p *Player) From() Kind {
	if s := p.source(); s != nil {
		return s.Kind()
	}
	return FromQueue
}

// Named is what to call whoever holds the card, and empty where it has no name.
func (p *Player) Named() string {
	if s := p.source(); s != nil {
		return s.Label()
	}
	return ""
}

// A pointer to the interface, because an interface holding a nil pointer is not itself nil.

// External gives the card and the transport to a source that has started playing, and the last to
// play keeps them once it stops.
//
// On playing, not on connecting: claiming at connection takes the card from whatever is sounding.
// nil clears it, for tests; a source that has gone calls Release.
func (p *Player) External(s Source) {
	if s == nil {
		p.sourceOwner().Clear()
	} else {
		p.sourceOwner().Set(s)
	}
	p.refresh()
}

// Release gives the card up, and only if this source holds it.
//
// The holder is compared, so a source has to be comparable: a pointer, or a struct of comparable
// fields.
func (p *Player) Release(s Source) {
	if p.sourceOwner().Release(s) {
		p.refresh()
	}
}

// Changed is how a source says it is doing something different now.
func (p *Player) Changed() { p.refresh() }

// source is whoever the card and the transport belong to: the last thing to have played.
//
// Nothing arbitrates between Home Assistant's queue and the rest. It is one of them and claims the
// card the same way.
func (p *Player) source() Source { s, _ := p.sourceOwner().Get(); return s }

// Sourced is who the card belongs to and whether anything holds it, for the control socket. Nobody
// holding it, the local queue being heard instead, and a holder with nothing to say look the same
// from outside.
func (p *Player) Sourced() (string, bool) {
	s, held := p.sourceOwner().Get()
	if held {
		return fmt.Sprintf("%T", s), true
	}
	return "the local queue", false
}

// Now is what the device is playing, whoever started it. The screen and the entity both read it.
func (p *Player) Now() Now {
	if s := p.source(); s != nil {
		return s.Now()
	}
	return p.queued()
}

// queued is what Home Assistant has this device playing, for its own Source and for a player that
// nothing has claimed yet.
//
// No metadata arrives with a url, so there is nothing here but whether it is going.
func (p *Player) queued() Now {
	playing, paused := p.stream.Playing()
	return Now{Playing: playing, Paused: paused, Can: CanPause | CanStop}
}
