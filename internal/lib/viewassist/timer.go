package viewassist

import "slices"

// The timer contract, not a timer. internal/feature/timer already runs ours; this is the shape View
// Assist expects one to be reported in, so the two can be spoken about in the same words.

// Kind is what a timer is for. View Assist keeps them apart because they are answered differently:
// a reminder says something, a command runs something.
type Kind string

const (
	KindAlarm    Kind = "alarm"
	KindTimer    Kind = "timer"
	KindReminder Kind = "reminder"
	KindCommand  Kind = "command"
)

// Kinds is every kind there is.
var Kinds = []Kind{KindAlarm, KindTimer, KindReminder, KindCommand}

// Known reports whether this is a kind View Assist defines.
func (k Kind) Known() bool { return slices.Contains(Kinds, k) }

// Status is where a timer is in its life.
type Status string

const (
	Running   Status = "running"
	Snoozed   Status = "snoozed"
	Expired   Status = "expired"
	Cancelled Status = "cancelled"
)

// Over reports whether the timer has finished, either way it could.
func (s Status) Over() bool { return s == Expired || s == Cancelled }

// Reckoning is how a timer's moment was given: a length from now, or a time of day.
type Reckoning string

const (
	Interval Reckoning = "TimerInterval"
	AtTime   Reckoning = "TimerTime"
)

// What happens to a timer, as the events View Assist broadcasts name it.
const (
	Started   = "started"
	Warning   = "warning"
	ExpiredAt = "expired"
	Snooze    = "snoozed"
	Cancel    = "cancelled"
)

// Event is the name of the event fired when something happens to a timer.
//
// Command timers are a separate stream, because a command firing is something to run rather than
// something to say, and anything listening for one wants only its own.
func Event(k Kind, what string) string {
	if k == KindCommand {
		return "va_timer_command_" + what
	}
	return "va_timer_" + what
}

// Warn is how long before a timer expires the warning fires, in seconds, when nothing says
// otherwise.
const Warn = 10
