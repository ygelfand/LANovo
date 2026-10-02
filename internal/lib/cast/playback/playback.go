// Package playback is the contract between something that produces audio and the device that plays
// it: the producer hands over a Source, the Output takes the speaker and the card for it.
package playback

import (
	"context"
	"io"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/surface"
)

type Branded interface {
	Logo() string
}

type Seeker interface {
	Seek(to time.Duration)
}

type Mark struct {
	From, To time.Duration
	RGB      uint32
}

type Marked interface {
	Marks() []Mark
}

type Pictures interface {
	Pictured() bool
	Pictures(ctx context.Context) (webm io.ReadCloser, from time.Duration, err error)
}

type Picture struct {
	H264          bool
	Width, Height int
	Next          func() (data []byte, at time.Duration, err error)
	Session       uint32
	Sealed        func() (data []byte, at time.Duration, crypt *surface.Crypt, err error)
}

type Live interface {
	Live(ctx context.Context) (Picture, error)
}

// Output is the device's speaker and player card, and the one owner of whether a source is paused
// and how far into it the room has heard.
type Output interface {
	Play(src Source)
	Stop(src Source)
	Handoff(src Source)

	Pause(src Source)
	Resume(src Source)

	// Position is how far into src the room has heard, and whether it is paused.
	Position(src Source) (at time.Duration, paused bool)

	// Behind is how much of what src has handed over has not been heard yet.
	Behind(src Source) time.Duration

	// Changed says src has new metadata to show.
	Changed(src Source)

	Attend(s *Session)
	Failed(err error)
}

type Session struct {
	Label    string
	Logo     string
	Pictured bool
}

// Source is something being played.
type Source interface {
	// Read fills pcm with interleaved 48 kHz stereo and says how many samples it wrote. It returns
	// 0 while there is nothing yet, and io.EOF at the end.
	Read(pcm []int16) (int, error)

	// Start is where in the media the first sample Read hands over sits.
	Start() time.Duration

	Showing() Showing
	Label() string

	// The card's buttons.
	Play()
	Pause()
	Stop()
	Next()
	Previous()
}

// Showing is what a Source says about itself for the card.
type Showing struct {
	Title  string
	Artist string
	Album  string

	// Art is a url, fetched by the output.
	Art string

	Mark string

	Length time.Duration

	LiveWithin time.Duration

	HasNext, HasPrevious bool

	// Upcoming is what plays next, nearest first, as far as the source knows.
	Upcoming []Upcoming
}

// Upcoming is one queued item as the card lists it.
type Upcoming struct {
	Title  string
	Artist string
	Length time.Duration

	// Play jumps to it, nil where the source cannot.
	Play func()
}
