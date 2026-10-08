package viewassist

import "slices"

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

var Answerable = []string{
	Navigate, SetState, AddStatusItem, RemoveStatus, ToggleMenu,
	SoundAlarm, CancelAlarm,
	SetTimer, CancelTimer, SnoozeTimer, GetTimers,
	BroadcastEvent,
}

var Elsewhere = []string{
	"load_view", "save_view", "load_asset", "save_asset",
	"set_master_config", "update_versions",
}

func Served(action string) bool { return slices.Contains(Answerable, action) }

type State struct {
	Title   string
	Message string
}

func (s State) Empty() bool { return s.Title == "" && s.Message == "" }
