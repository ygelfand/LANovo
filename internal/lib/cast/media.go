package cast

import (
	"encoding/json"
	"fmt"
	"time"
)

// The media namespace: what is playing, and the transport controls.
//
// Times here are seconds as a floating point number, which is what the protocol carries and not
// what the rest of this tree uses. They are converted at the edge so nothing above has to remember
// which unit it is holding.

// What the player is doing.
const (
	StateIdle      = "IDLE"
	StateBuffering = "BUFFERING"
	StatePlaying   = "PLAYING"
	StatePaused    = "PAUSED"
)

// Why it went idle, which a sender shows differently for each.
const (
	IdleFinished    = "FINISHED"
	IdleCancelled   = "CANCELLED"
	IdleInterrupted = "INTERRUPTED"
	IdleError       = "ERROR"
)

// How the stream arrives. Buffered can be sought, live cannot.
const (
	StreamBuffered = "BUFFERED"
	StreamLive     = "LIVE"
	StreamNone     = "NONE"
)

// What kind of thing the metadata describes, which decides which fields a sender reads.
const (
	MetadataGeneric = 0
	MetadataMovie   = 1
	MetadataTVShow  = 2
	MetadataMusic   = 3
	MetadataPhoto   = 4
)

// The commands a sender may offer, as the bits that say so.
//
// A sender greys out what is not here, so this is the difference between a pause button that works
// and one that is not drawn at all.
const (
	CommandPause        = 1 << 0
	CommandSeek         = 1 << 1
	CommandStreamVolume = 1 << 2
	CommandStreamMute   = 1 << 3
	CommandQueueNext    = 1 << 6
	CommandQueuePrev    = 1 << 7
)

// Speaker is what this device can do about media a sender gives it: pause, seek, and its own
// volume. No queue, because nothing here implements one yet and claiming it gets a next button
// that does nothing.
const Speaker = CommandPause | CommandSeek | CommandStreamVolume | CommandStreamMute

// Image is artwork.
type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// Metadata is what is playing, as a sender described it.
type Metadata struct {
	MetadataType int     `json:"metadataType"`
	Title        string  `json:"title,omitempty"`
	Subtitle     string  `json:"subtitle,omitempty"`
	Artist       string  `json:"artist,omitempty"`
	AlbumName    string  `json:"albumName,omitempty"`
	AlbumArtist  string  `json:"albumArtist,omitempty"`
	Composer     string  `json:"composer,omitempty"`
	TrackNumber  int     `json:"trackNumber,omitempty"`
	DiscNumber   int     `json:"discNumber,omitempty"`
	Images       []Image `json:"images,omitempty"`
	ReleaseDate  string  `json:"releaseDate,omitempty"`
}

// Media is the thing itself.
type Media struct {
	// ContentID is the url to play, for everything this device will ever be given.
	ContentID   string `json:"contentId"`
	StreamType  string `json:"streamType"`
	ContentType string `json:"contentType"`

	Metadata Metadata `json:"metadata,omitempty"`

	// Duration is in seconds, and zero for a live stream that has none.
	Duration float64 `json:"duration,omitempty"`

	// CustomData is whatever the sender put there. Kept as raw JSON and handed back untouched: it
	// is the sender's own, and parsing it would only be a way to lose it.
	CustomData json.RawMessage `json:"customData,omitempty"`
}

// Length is the duration as the rest of this tree counts time.
func (m Media) Length() time.Duration { return seconds(m.Duration) }

// Title is the best name for what is playing, for something that has to draw one.
func (m Media) Title() string {
	if m.Metadata.Title != "" {
		return m.Metadata.Title
	}
	return m.ContentID
}

// seconds turns the protocol's floating point seconds into a duration.
func seconds(v float64) time.Duration { return time.Duration(v * float64(time.Second)) }

// asSeconds turns one back.
func asSeconds(d time.Duration) float64 { return d.Seconds() }

// MediaStatus is one entry in a status list.
type MediaStatus struct {
	// MediaSessionID identifies this piece of media, and a sender addresses its transport commands
	// to it. It changes with every LOAD.
	MediaSessionID int `json:"mediaSessionId"`

	PlaybackRate float64 `json:"playbackRate"`
	PlayerState  string  `json:"playerState"`

	// CurrentTime is in seconds. A sender runs its own clock between statuses, so this only has to
	// be right when it is sent rather than continuously.
	CurrentTime float64 `json:"currentTime"`

	SupportedMediaCommands int    `json:"supportedMediaCommands"`
	Volume                 Volume `json:"volume"`

	// Media is sent in full on the status that follows a LOAD and left out afterwards, because it
	// does not change and a sender already has it. It is a few kilobytes with artwork urls.
	Media *Media `json:"media,omitempty"`

	// IdleReason is set only when the state is idle, and says whether it finished or was stopped.
	IdleReason string `json:"idleReason,omitempty"`

	CurrentItemID int `json:"currentItemId,omitempty"`
}

// Elapsed is where the media has got to, as the rest of this tree counts time.
func (s MediaStatus) Elapsed() time.Duration { return seconds(s.CurrentTime) }

// mediaStatusEnvelope is how a status goes out.
//
// The status is a list, always, even for the one thing this device plays. A sender reading a
// single object where it expects an array treats the whole message as malformed and shows nothing.
type mediaStatusEnvelope struct {
	Header
	Status []MediaStatus `json:"status"`
}

// MediaStatusPayload is a MEDIA_STATUS answering a request, or unsolicited when the id is zero.
func MediaStatusPayload(request int, s ...MediaStatus) string {
	// Not nil, so it marshals as [] rather than null. A sender reads null as malformed where it
	// reads an empty list as nothing playing.
	if s == nil {
		s = []MediaStatus{}
	}
	return payload(mediaStatusEnvelope{
		Header: Header{Type: TypeMediaStatus, RequestID: request},
		Status: s,
	})
}

// LoadRequest is a sender giving this device something to play.
type LoadRequest struct {
	Header

	Media Media `json:"media"`

	// Autoplay is whether to start immediately. A sender that sends false wants it loaded and
	// paused, which is not the same as not sending the field at all — so it is a pointer, and
	// absent means the protocol's default of true.
	Autoplay *bool `json:"autoplay,omitempty"`

	// CurrentTime is where to start, in seconds.
	CurrentTime float64 `json:"currentTime,omitempty"`

	CustomData json.RawMessage `json:"customData,omitempty"`
}

// Starts reports whether this load should begin playing, which is true unless the sender said not.
func (r LoadRequest) Starts() bool { return r.Autoplay == nil || *r.Autoplay }

// ParseLoad reads a LOAD.
func ParseLoad(p string) (LoadRequest, error) {
	var r LoadRequest
	if err := json.Unmarshal([]byte(p), &r); err != nil {
		return r, fmt.Errorf("cast: reading a load: %w", err)
	}
	if r.Media.ContentID == "" {
		return r, fmt.Errorf("cast: a load naming nothing to play")
	}
	return r, nil
}

// MediaRequest is a transport command, which names the session it is about.
type MediaRequest struct {
	Header

	MediaSessionID int `json:"mediaSessionId"`

	// CurrentTime is where to seek to, for a SEEK. Seconds.
	CurrentTime float64 `json:"currentTime,omitempty"`

	// ResumeState is what a seek should do about playing: PLAYBACK_START or PLAYBACK_PAUSE, and
	// empty to leave it as it was.
	ResumeState string `json:"resumeState,omitempty"`
}

// What a seek may ask for.
const (
	ResumePlay  = "PLAYBACK_START"
	ResumePause = "PLAYBACK_PAUSE"
)

// ParseMediaRequest reads a PLAY, PAUSE, STOP, SEEK or GET_STATUS.
func ParseMediaRequest(p string) (MediaRequest, error) {
	var r MediaRequest
	if err := json.Unmarshal([]byte(p), &r); err != nil {
		return r, fmt.Errorf("cast: reading a media command: %w", err)
	}
	return r, nil
}

// Seek is where a seek asked to go.
func (r MediaRequest) Seek() time.Duration { return seconds(r.CurrentTime) }

// LoadFailed refuses a load, which is its own type rather than an invalid request.
func LoadFailed(request int) string {
	return payload(Header{Type: TypeLoadFailed, RequestID: request})
}
