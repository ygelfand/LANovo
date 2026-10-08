package media

import (
	"fmt"

	"github.com/ygelfand/libcountertop/pkg/display/mark"
	sharedsource "github.com/ygelfand/libcountertop/pkg/media/source"

	"github.com/ygelfand/LANovo/internal/ui"
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

func (p *Player) From() Kind {
	if s := p.source(); s != nil {
		return s.Kind()
	}
	return FromQueue
}

func (p *Player) Named() string {
	if s := p.source(); s != nil {
		return s.Label()
	}
	return ""
}

// An interface holding a nil pointer is not itself nil.

func (p *Player) External(s Source) {
	if s == nil {
		p.sourceOwner().Clear()
	} else {
		p.sourceOwner().Set(s)
	}
	p.refresh()
}

func (p *Player) Release(s Source) {
	if p.sourceOwner().Release(s) {
		p.refresh()
	}
}

func (p *Player) Changed() { p.refresh() }

func (p *Player) source() Source { s, _ := p.sourceOwner().Get(); return s }

func (p *Player) Sourced() (string, bool) {
	s, held := p.sourceOwner().Get()
	if held {
		return fmt.Sprintf("%T", s), true
	}
	return "the local queue", false
}

func (p *Player) Now() Now {
	if s := p.source(); s != nil {
		return s.Now()
	}
	return p.queued()
}

func (p *Player) queued() Now {
	playing, paused := p.stream.Playing()
	return Now{Playing: playing, Paused: paused, Can: CanPause | CanStop}
}
