package config

import "github.com/ygelfand/LANovo/internal/lib/say"

import "time"

// Clock is how the dashboard draws the time.
//
// Its own area rather than part of Screen, because Screen is the panel — how bright it is, which
// way the drawer comes in — and this is what is drawn on it. The two change for different reasons
// and there is more coming here than there is there.
type Clock struct {
	Face Face `json:"face"`

	// Date puts the day under the time. On a device that sits in a room it is most of what the
	// screen is for besides the clock, and it is also the first thing somebody wanting only the
	// time will turn off.
	Date bool `json:"date"`

	// Position is where on the glass the clock sits, and Size is how much of it the clock takes.
	//
	// A pair, and the reason neither is the other: position without size is a clock that can only
	// be large, and size without position is a small clock stranded in the middle of a tall panel.
	Position Position `json:"position"`
	Size     Size     `json:"size"`

	// Ink is the color the clock is drawn in, over whatever theme is on.
	Ink Ink `json:"ink"`
}

// Delay is how long the device waits, with nobody touching it, before the idle screen comes up.
type Delay string

const (
	DelayNever Delay = "never"
	Delay30s   Delay = "30s"
	Delay1m    Delay = "1m"
	Delay2m    Delay = "2m"
	Delay3m    Delay = "3m"
	Delay5m    Delay = "5m"
	Delay10m   Delay = "10m"
	Delay15m   Delay = "15m"
)

// DefaultDelay is a minute: long enough that somebody reading the screen is not interrupted, short
// enough that a device left alone settles while they are still in the room to see it.
const DefaultDelay = Delay1m

func (d Delay) Label() string {
	switch d {
	case DelayNever:
		return say.T("delay.never")
	case Delay30s:
		return say.T("delay.30s")
	case Delay1m:
		return say.T("delay.1m")
	case Delay2m:
		return say.T("delay.2m")
	case Delay3m:
		return say.T("delay.3m")
	case Delay5m:
		return say.T("delay.5m")
	case Delay10m:
		return say.T("delay.10m")
	case Delay15m:
		return say.T("delay.15m")
	}
	return string(d)
}

// After is the wait as a duration. Never is zero, which every caller reads as not sleeping at all
// rather than as sleeping immediately.
func (d Delay) After() time.Duration {
	switch d {
	case Delay30s:
		return 30 * time.Second
	case Delay1m:
		return time.Minute
	case Delay2m:
		return 2 * time.Minute
	case Delay3m:
		return 3 * time.Minute
	case Delay5m:
		return 5 * time.Minute
	case Delay10m:
		return 10 * time.Minute
	case Delay15m:
		return 15 * time.Minute
	}
	return 0
}

// Delays is every wait that can be chosen, shortest first, with Never at the end: it is the one
// somebody picks on purpose rather than the one they are looking for.
func Delays() []Delay {
	return []Delay{Delay30s, Delay1m, Delay2m, Delay5m, Delay15m, DelayNever}
}

func MediaDelays() []Delay {
	return []Delay{Delay1m, Delay3m, Delay5m, Delay10m, Delay15m, DelayNever}
}

// Position is the part of the screen the clock is given.
//
// What size is for on a device that stands on something: a clock read from a bed wants to be low,
// one on a kitchen shelf high and out of the way of whatever is under it.
type Position string

const (
	PositionTop    Position = "top"
	PositionCenter Position = "center"
	PositionBottom Position = "bottom"
)

const DefaultPosition = PositionCenter

func (p Position) Label() string {
	switch p {
	case PositionTop:
		return say.T("position.top")
	case PositionCenter:
		return say.T("position.center")
	case PositionBottom:
		return say.T("position.bottom")
	}
	return string(p)
}

// Positions is every place the clock can sit, in the order they are on the screen.
func Positions() []Position {
	return []Position{PositionTop, PositionCenter, PositionBottom}
}

// Size is how much of the room it is given the clock fills.
//
// A share rather than a height in pixels. Every face already fits itself to the box it is handed
// and the two orientations are different shapes, so a number of pixels would mean one thing
// standing up and another lying down. A share means the same thing both ways.
type Size string

const (
	SizeNano   Size = "nano"
	SizeMicro  Size = "micro"
	SizeMini   Size = "mini"
	SizeSmall  Size = "small"
	SizeMedium Size = "medium"
	SizeLarge  Size = "large"
)

// DefaultSize is large. This is a clock on a shelf read from across a room, and the face filling
// what it is given is what the device did before there was a choice.
const DefaultSize = SizeLarge

func (s Size) Label() string {
	switch s {
	case SizeNano:
		return say.T("size.nano")
	case SizeMicro:
		return say.T("size.micro")
	case SizeMini:
		return say.T("size.mini")
	case SizeSmall:
		return say.T("size.small")
	case SizeMedium:
		return say.T("size.medium")
	case SizeLarge:
		return say.T("size.large")
	}
	return string(s)
}

// Share is how much of each side of the box the clock keeps.
//
// Both sides, so the clock scales rather than stretches. Small is not half: a face fits itself to
// whichever side runs out first, so halving the box is nearer a quarter of the clock, and small
// still has to be legible from the far side of a room.
func (s Size) Share() float64 {
	switch s {
	case SizeNano:
		return 0.16
	case SizeMicro:
		return 0.26
	case SizeMini:
		return 0.40
	case SizeSmall:
		return 0.58
	case SizeMedium:
		return 0.78
	}
	return 1
}

// Sizes is every size, smallest first.
func Sizes() []Size {
	return []Size{SizeNano, SizeMicro, SizeMini, SizeSmall, SizeMedium, SizeLarge}
}

// DefaultFace is what the device has always drawn, so an upgrade changes nothing about a device
// somebody has already put on a shelf.
const DefaultFace = FacePlain

// DefaultDate is on, for the same reason.
const DefaultDate = true

func defaultClock() Clock {
	return Clock{
		Face:     DefaultFace,
		Date:     DefaultDate,
		Position: DefaultPosition,
		Size:     DefaultSize,
		Ink:      DefaultInk,
	}
}

// Face is a way of drawing the clock. The name is written to the file and branched on; the label is
// what Home Assistant shows.
//
// A face is a layout and the numerals it is drawn in together, not two settings multiplied: offering
// both is twenty combinations nobody asked for, and the pairs that read well are a short list
// somebody chose.
type Face string

const (
	// FacePlain is the time across the middle with the date under it.
	FacePlain Face = "plain"

	// FaceStack is the hour over the minutes, as large as the panel allows. The portrait answer:
	// one line of time leaves half a tall screen empty.
	FaceStack Face = "stack"

	// FaceCards is the hour and the minutes on two panels with a seam across each, which is what
	// this device showed before we took it over.
	FaceCards Face = "cards"

	// FaceAnalog is hands, which is the one that reads as a clock from a doorway rather than as a
	// readout to be looked at.
	FaceAnalog Face = "analog"

	// FaceAnalogSeconds is the same face with a second hand. Its own choice rather than a switch
	// beside the faces, because a switch that applied to one of them and did nothing to the rest is
	// a setting somebody has to learn the exception to. It is also the only face that costs
	// anything to leave running: it redraws every second.
	FaceAnalogSeconds Face = "analog-seconds"

	// FaceSegments is a seven segment display, with the segments that are off drawn faintly, which
	// is what makes it a device with lamps in it rather than a typeface.
	FaceSegments Face = "segments"

	// FaceWords says the time the way somebody asked would say it. The slowest to read, which is
	// the choice being offered rather than a fault in it.
	FaceWords Face = "words"

	// FaceOverlap is the hour and the minutes run into each other, very heavy. Barely legible at a
	// glance and meant to be: a clock to look at rather than one that announces itself.
	FaceOverlap Face = "overlap"
)

func (f Face) Label() string {
	switch f {
	case FaceNone:
		return say.T("face.none")
	case FacePlain:
		return say.T("face.plain")
	case FaceStack:
		return say.T("face.stack")
	case FaceCards:
		return say.T("face.cards")
	case FaceAnalog:
		return say.T("face.analog")
	case FaceAnalogSeconds:
		return say.T("face.analog-seconds")
	case FaceSegments:
		return say.T("face.segments")
	case FaceWords:
		return say.T("face.words")
	case FaceOverlap:
		return say.T("face.overlap")
	}
	return string(f)
}

// Faces is every face the device can draw, in the order they are offered.
//
// Hand-kept here rather than gathered from whatever registered itself, because config cannot import
// the drawing and the drawing already imports config. A name here with nothing to draw it is what
// the face package's own test exists to catch.
func Faces() []Face {
	return []Face{
		FacePlain, FaceStack, FaceCards,
		FaceAnalog, FaceAnalogSeconds, FaceSegments,
		FaceWords, FaceOverlap,
	}
}

// ClockWriter changes how the time is drawn.
type ClockWriter struct{ st *Store }

func (w ClockWriter) Face(v Face) error {
	return w.st.Update(func(c *Config) { c.Clock.Face = v })
}

func (w ClockWriter) Date(v bool) error {
	return w.st.Update(func(c *Config) { c.Clock.Date = v })
}

func (w ClockWriter) Position(v Position) error {
	return w.st.Update(func(c *Config) { c.Clock.Position = v })
}

func (w ClockWriter) Size(v Size) error {
	return w.st.Update(func(c *Config) { c.Clock.Size = v })
}

func (w ClockWriter) Ink(v Ink) error {
	return w.st.Update(func(c *Config) { c.Clock.Ink = v })
}
