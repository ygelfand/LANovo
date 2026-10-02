package viewassist

import "slices"

// The actions View Assist calls on a satellite, by the names its own service calls use.
//
// Only some of these are ours to answer. A satellite is asked to navigate, to show a title and a
// message, and to carry status icons; loading and saving views and assets is the integration
// managing files on the Home Assistant side and means nothing here. The ones we do not answer are
// still named, because knowing what exists is what says which list a missing feature belongs on.

// Answerable is an action a satellite can serve.
const (
	Navigate       = "navigate"
	SetState       = "set_state"
	AddStatusItem  = "add_status_item"
	RemoveStatus   = "remove_status_item"
	ToggleMenu     = "toggle_menu"
	SoundAlarm     = "sound_alarm"
	CancelAlarm    = "cancel_sound_alarm"
	SetTimer       = "set_timer"
	CancelTimer    = "cancel_timer"
	SnoozeTimer    = "snooze_timer"
	GetTimers      = "get_timers"
	BroadcastEvent = "broadcast_event"
)

// Answerable is every action worth a satellite serving, in the order a reader would want them: what
// is on screen, then what it is saying, then timers.
var Answerable = []string{
	Navigate, SetState, AddStatusItem, RemoveStatus, ToggleMenu,
	SoundAlarm, CancelAlarm,
	SetTimer, CancelTimer, SnoozeTimer, GetTimers,
	BroadcastEvent,
}

// Elsewhere is the rest of View Assist's service surface: the integration managing views, assets
// and its own configuration on the Home Assistant side. Named so that a search for an action finds
// out it exists and is not ours, rather than finding nothing.
var Elsewhere = []string{
	"load_view", "save_view", "load_asset", "save_asset",
	"set_master_config", "update_versions",
}

// Served reports whether a satellite is the thing that answers this action.
func Served(action string) bool { return slices.Contains(Answerable, action) }

// State is what set_state puts on the screen. Both are optional and either alone is meaningful: a
// title with no message is a heading, a message with no title is a line of text.
type State struct {
	Title   string
	Message string
}

// Empty reports whether there is nothing to show, which is how a screen is cleared.
func (s State) Empty() bool { return s.Title == "" && s.Message == "" }
