package viewassist

import "slices"

type Kind string

const (
	KindAlarm    Kind = "alarm"
	KindTimer    Kind = "timer"
	KindReminder Kind = "reminder"
	KindCommand  Kind = "command"
)

var Kinds = []Kind{KindAlarm, KindTimer, KindReminder, KindCommand}

func (k Kind) Known() bool { return slices.Contains(Kinds, k) }

type Status string

const (
	Running   Status = "running"
	Snoozed   Status = "snoozed"
	Expired   Status = "expired"
	Cancelled Status = "cancelled"
)

func (s Status) Over() bool { return s == Expired || s == Cancelled }

type Reckoning string

const (
	Interval Reckoning = "TimerInterval"
	AtTime   Reckoning = "TimerTime"
)

const (
	Started   = "started"
	Warning   = "warning"
	ExpiredAt = "expired"
	Snooze    = "snoozed"
	Cancel    = "cancelled"
)

func Event(k Kind, what string) string {
	if k == KindCommand {
		return "va_timer_command_" + what
	}
	return "va_timer_" + what
}

const Warn = 10
