package media

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/libcountertop/pkg/display/mark"
)

// Source is audio this player did not start, playing through the same speaker: a group the room has
// joined. There is one media player entity either way, so the transport has to reach whoever is
// playing.
type Seeker interface {
	Seek(to time.Duration)
	CanSeek() bool
}

type Source interface {
	Play()
	Pause()
	Stop()
	Next()
	Previous()

	// Now is pulled when the entity or the screen redraws, not pushed.
	Now() Now

	// Kind is which of them this is, for the mark on the card.
	Kind() Kind

	// Label is what to call it: the phone's own name, the group's. Data rather than interface copy,
	// so it is not translated. Empty where the source has no name to give.
	Label() string
}

// Opener is a source with a player of its own to show, such as a video. Open reports whether it
// put one up; a source that did not gets the card.
type Opener interface {
	Open() bool
}

// Kind is where the audio came from.
type Kind uint8

const (
	FromQueue Kind = iota
	FromBluetooth
	FromGroup
	FromCast
)

// Icon is the mark shown on the card.
func (k Kind) Icon() ui.Icon {
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

// Now is what is playing, for the entity and for the screen.
type Now struct {
	Playing bool
	Paused  bool
	Hold    bool

	// Empty where the source does not know, which is anything played from a url.
	Title  string
	Artist string
	Album  string

	// Art is the album art, nil where there is none. Decoded by whoever received it, since it
	// arrives once and is drawn every redraw.
	Art *ui.Image

	Mark *ui.Image

	// Elapsed is how far into the track playback has reached, and Length how long it runs for. A
	// zero Length is a source that does not say, and the card draws no bar for it.
	Elapsed time.Duration
	Length  time.Duration

	LiveWithin time.Duration

	// Queue is what follows, nearest first. Sources that have no list leave it empty.
	Queue []Track

	// Can is the transport to offer.
	Can Controls
}

// Track is one entry in the queue.
type Track struct {
	Title  string
	Artist string
	Length time.Duration

	// Art is the entry's own artwork, nil where none was fetched.
	Art *ui.Image

	// Play jumps to this entry, and is nil where the source cannot.
	//
	// The source fills it in while building the list, so whatever names an entry to the far end —
	// a uid and the counter the listing came with — is captured with the listing it belongs to
	// rather than looked up again later against one that has moved.
	Play func()
}

// Controls is which transport a source answers to.
type Controls uint8

const (
	CanPause Controls = 1 << iota
	CanStop
	CanNext
	CanPrevious
)

// Has reports whether every control in want is offered.
func (c Controls) Has(want Controls) bool { return c&want == want }

// A pointer to the interface, because an interface holding a nil pointer is not itself nil.
var externalSource atomic.Pointer[Source]

// External gives the card and the transport to a source that has started playing, and the last to
// play keeps them once it stops.
//
// On playing, not on connecting: claiming at connection takes the card from whatever is sounding.
// nil clears it, for tests; a source that has gone calls Release.
func (p *Player) External(s Source) {
	if s == nil {
		externalSource.Store(nil)
	} else {
		externalSource.Store(&s)
	}
	p.refresh()
}

// Release gives the card up, and only if this source holds it.
//
// The holder is compared, so a source has to be comparable: a pointer, or a struct of comparable
// fields.
func (p *Player) Release(s Source) {
	cur := externalSource.Load()
	if cur == nil || *cur != s {
		return
	}

	externalSource.Store(nil)
	slog.Info("the player card was given up", "by", fmt.Sprintf("%T", s))
	p.refresh()
}

// Changed is how a source says it is doing something different now.
func (p *Player) Changed() { p.refresh() }

// source is whoever the card and the transport belong to: the last thing to have played.
//
// Nothing arbitrates between Home Assistant's queue and the rest. It is one of them and claims the
// card the same way.
func (p *Player) source() Source {
	s := externalSource.Load()
	if s == nil {
		return nil
	}
	return *s
}

// Sourced is who the card belongs to and whether anything holds it, for the control socket. Nobody
// holding it, the local queue being heard instead, and a holder with nothing to say look the same
// from outside.
func (p *Player) Sourced() (source string, external bool) {
	held := externalSource.Load()

	if s := p.source(); s != nil {
		return fmt.Sprintf("%T", s), held != nil
	}
	return "the local queue", held != nil
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
	p.mu.Lock()
	defer p.mu.Unlock()

	return Now{Playing: p.playing, Can: CanStop}
}
