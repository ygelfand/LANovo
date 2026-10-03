package cast

import (
	"encoding/json"
	"fmt"
)

// The namespaces and what travels on them.
//
// Every payload is JSON with a "type" saying what it is, and most carry a requestId that pairs an
// answer with its question. A request answered with the wrong id, or with none, is a sender that
// waits out its timeout and gives up — so the id is echoed back on everything that had one.

// The namespaces this device answers.
const (
	NSConnection = "urn:x-cast:com.google.cast.tp.connection"
	NSHeartbeat  = "urn:x-cast:com.google.cast.tp.heartbeat"
	NSReceiver   = "urn:x-cast:com.google.cast.receiver"
	NSMedia      = "urn:x-cast:com.google.cast.media"
	NSDeviceAuth = "urn:x-cast:com.google.cast.tp.deviceauth"
)

// The message types, by namespace.
const (
	TypeConnect    = "CONNECT"
	TypeClose      = "CLOSE"
	TypePing       = "PING"
	TypePong       = "PONG"
	TypeGetStatus  = "GET_STATUS"
	TypeStatus     = "RECEIVER_STATUS"
	TypeLaunch     = "LAUNCH"
	TypeStop       = "STOP"
	TypeLaunchFail = "LAUNCH_ERROR"
	TypeInvalid    = "INVALID_REQUEST"
)

// The media namespace's own.
const (
	TypeLoad        = "LOAD"
	TypePlay        = "PLAY"
	TypePause       = "PAUSE"
	TypeSeek        = "SEEK"
	TypeStopMedia   = "STOP"
	TypeMediaStatus = "MEDIA_STATUS"
	TypeSetVolume   = "SET_VOLUME"
	TypeQueueNext   = "QUEUE_NEXT"
	TypeQueuePrev   = "QUEUE_PREV"
	TypeQueueUpdate = "QUEUE_UPDATE"
	TypeLoadFailed  = "LOAD_FAILED"
)

// Header is what every payload has, and sometimes all it has.
type Header struct {
	Type string `json:"type"`

	// RequestID pairs an answer with its question. Zero means unsolicited — a status the receiver
	// sent because something changed rather than because it was asked.
	RequestID int `json:"requestId"`
}

// Kind reads the type out of a payload without caring what else is in it, which is what says how to
// parse the rest.
func Kind(payload string) (Header, error) {
	var h Header
	if err := json.Unmarshal([]byte(payload), &h); err != nil {
		return Header{}, fmt.Errorf("cast: reading a payload: %w", err)
	}
	if h.Type == "" {
		return Header{}, fmt.Errorf("cast: a payload with no type")
	}
	return h, nil
}

// payload marshals a value as a message payload. The values here are all built in this package, so
// a marshalling failure is a bug rather than bad input and there is nothing useful to do with it.
func payload(v any) string {
	out, err := json.Marshal(v)
	if err != nil {
		return `{"type":"INVALID_REQUEST","reason":"INVALID_PARAMS"}`
	}
	return string(out)
}

// Volume is how loud the receiver is, as a sender sees it.
type Volume struct {
	// Level is 0 to 1. A pointer so that a SET_VOLUME carrying only a mute does not read as a
	// request to set the level to silence.
	Level *float64 `json:"level,omitempty"`
	Muted *bool    `json:"muted,omitempty"`

	// StepInterval is how much a relative volume button moves it, which senders use to decide
	// whether to show a slider or a pair of buttons.
	StepInterval float64 `json:"stepInterval,omitempty"`

	// ControlType is "attenuation" for a device with its own volume, "fixed" for one whose output
	// cannot change, "master" for one that sets a system volume.
	ControlType string `json:"controlType,omitempty"`
}

// Application is one thing running on the receiver.
type Application struct {
	AppID          string `json:"appId"`
	UniversalAppID string `json:"universalAppId"`
	AppType        string `json:"appType"`
	IconURL        string `json:"iconUrl,omitempty"`

	LaunchedFromCloud bool `json:"launchedFromCloud"`

	// SessionID identifies this run of it, and TransportID is the endpoint to address messages to
	// once it is running. They are usually the same string, and senders read both.
	SessionID   string `json:"sessionId"`
	TransportID string `json:"transportId"`

	DisplayName string `json:"displayName"`
	StatusText  string `json:"statusText"`

	// Namespaces is what the application answers, which is how a sender knows it may speak the
	// media namespace to it rather than guessing.
	Namespaces []Namespace `json:"namespaces"`

	IsIdleScreen bool `json:"isIdleScreen"`
}

// Namespace is one entry in an application's list.
type Namespace struct {
	Name string `json:"name"`
}

// ReceiverStatus is what the receiver is doing.
type ReceiverStatus struct {
	// Applications is empty when nothing is running, and a sender reads that as the device being
	// free rather than as an error.
	Applications []Application `json:"applications,omitempty"`

	Volume Volume `json:"volume"`

	IsActiveInput bool `json:"isActiveInput"`
	IsStandBy     bool `json:"isStandBy"`
}

// statusEnvelope is how a receiver status goes on the wire: the header, then the status under a
// key of its own rather than beside it.
type statusEnvelope struct {
	Header
	Status ReceiverStatus `json:"status"`
}

// Status is a RECEIVER_STATUS payload answering a request, or unsolicited when the id is zero.
func Status(request int, s ReceiverStatus) string {
	return payload(statusEnvelope{
		Header: Header{Type: TypeStatus, RequestID: request},
		Status: s,
	})
}

// Pong answers a heartbeat. A sender that does not get one inside its own timeout drops the
// connection, so this is the cheapest message here and the one that must never be missed.
func Pong() string { return payload(Header{Type: TypePong}) }

// Ping is the other direction, for a receiver checking a sender is still there.
func Ping() string { return payload(Header{Type: TypePing}) }

// Why a request was refused.
const (
	ReasonInvalidParams  = "INVALID_PARAMS"
	ReasonNotSupported   = "NOT_SUPPORTED"
	ReasonUnknownAppID   = "UNKNOWN_APP_ID"
	ReasonInvalidCommand = "INVALID_COMMAND"
)

// refusal is an INVALID_REQUEST with a reason.
type refusal struct {
	Header
	Reason string `json:"reason"`
}

// Invalid refuses a request.
//
// Answered rather than ignored. A sender waiting on a reply holds the connection and eventually
// drops the whole thing, which reads in the room as the device refusing to be cast to at all.
func Invalid(request int, reason string) string {
	return payload(refusal{
		Header: Header{Type: TypeInvalid, RequestID: request},
		Reason: reason,
	})
}

// LaunchError refuses a launch, which is its own type rather than an invalid request.
func LaunchError(request int, reason string) string {
	return payload(refusal{
		Header: Header{Type: TypeLaunchFail, RequestID: request},
		Reason: reason,
	})
}

// LaunchRequest is a sender asking for an application.
type LaunchRequest struct {
	Header
	AppID string `json:"appId"`
}

// StopRequest is a sender asking for one to go away. The session is which one, and is empty for a
// sender that means whatever is running.
type StopRequest struct {
	Header
	SessionID string `json:"sessionId"`
}

// SetVolumeRequest is a sender moving the volume.
type SetVolumeRequest struct {
	Header
	Volume Volume `json:"volume"`
}

// ParseLaunch reads a LAUNCH.
func ParseLaunch(p string) (LaunchRequest, error) {
	var r LaunchRequest
	if err := json.Unmarshal([]byte(p), &r); err != nil {
		return r, fmt.Errorf("cast: reading a launch: %w", err)
	}
	if r.AppID == "" {
		return r, fmt.Errorf("cast: a launch naming no application")
	}
	return r, nil
}

// ParseStop reads a STOP.
func ParseStop(p string) (StopRequest, error) {
	var r StopRequest
	if err := json.Unmarshal([]byte(p), &r); err != nil {
		return r, fmt.Errorf("cast: reading a stop: %w", err)
	}
	return r, nil
}

// ParseSetVolume reads a SET_VOLUME.
//
// Level and Muted are pointers, so a request that carries only one of them does not read as a
// request to set the other to zero — which is the difference between muting a speaker and turning
// it all the way down.
func ParseSetVolume(p string) (SetVolumeRequest, error) {
	var r SetVolumeRequest
	if err := json.Unmarshal([]byte(p), &r); err != nil {
		return r, fmt.Errorf("cast: reading a volume: %w", err)
	}
	if r.Volume.Level == nil && r.Volume.Muted == nil {
		return r, fmt.Errorf("cast: a volume request setting neither level nor mute")
	}
	if l := r.Volume.Level; l != nil && (*l < 0 || *l > 1) {
		return r, fmt.Errorf("cast: a volume of %v, which is not between nought and one", *l)
	}
	return r, nil
}

// Connect opens a conversation with an endpoint, and Close ends one.
func Connect() string { return payload(Header{Type: TypeConnect}) }
func Close() string   { return payload(Header{Type: TypeClose}) }
