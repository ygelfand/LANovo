package viewassist

import "testing"

func TestAPathNamesItsView(t *testing.T) {
	for _, at := range []struct {
		path string
		want View
	}{
		{"/viewassist/clock", Clock},
		{"/viewassist/weather", Weather},
		{"viewassist/music", Music},
		{"/viewassist/clock/", Clock},
		{"/somewhere-else/clock", Clock},
		{"/deeply/nested/dashboard/alarm", Alarm},
		{"CLOCK", Clock},
	} {
		got, ok := Navigated(at.path)
		if !ok {
			t.Errorf("%q named no view, want %q", at.path, at.want)
			continue
		}
		if got != at.want {
			t.Errorf("%q named %q, want %q", at.path, got, at.want)
		}
	}
}

func TestAPathThatIsNotAViewIsNotOne(t *testing.T) {
	for _, path := range []string{
		"", "/", "///",
		"/viewassist/something-else",
		"/lovelace/0",
		"https://example.com/page",
	} {
		if got, ok := Navigated(path); ok {
			t.Errorf("%q named the view %q, want no view", path, got)
		}
	}
}

func TestEveryViewsPathNamesItBack(t *testing.T) {
	for _, v := range Views {
		got, ok := Navigated(v.Path())
		if !ok {
			t.Errorf("%q has the path %q, which names no view", v, v.Path())
			continue
		}
		if got != v {
			t.Errorf("%q has the path %q, which names %q", v, v.Path(), got)
		}
	}
}

func TestOnlyTheWebpageIsNotDrawable(t *testing.T) {
	for _, v := range Views {
		if v.Drawable() == (v == Webpage) {
			t.Errorf("%q: drawable is %v", v, v.Drawable())
		}
	}
	if View("invented").Drawable() {
		t.Error("a view this build does not have reports as drawable")
	}
}

func TestHoldAndCycleDoNotIdle(t *testing.T) {
	for _, m := range []Mode{Normal, MusicMode, Night, DoNotDisturb} {
		if !m.Idles() {
			t.Errorf("%q does not idle, want it to", m)
		}
	}
	for _, m := range []Mode{Hold, Cycle} {
		if m.Idles() {
			t.Errorf("%q idles, want it held", m)
		}
	}
}

func TestOnlyDoNotDisturbRefusesToWake(t *testing.T) {
	for _, m := range []Mode{Normal, MusicMode, Hold, Cycle, Night} {
		if !m.Wakes() {
			t.Errorf("%q will not wake the screen, want it to", m)
		}
	}
	if DoNotDisturb.Wakes() {
		t.Error("do not disturb wakes the screen")
	}
}

func TestAnUnknownModeBehavesLikeNormal(t *testing.T) {
	m := Mode("something-new")

	if !m.Idles() || !m.Wakes() {
		t.Errorf("an unknown mode idles=%v wakes=%v, want both", m.Idles(), m.Wakes())
	}
}

func TestCommandTimersFireTheirOwnEvents(t *testing.T) {
	if got, want := Event(KindTimer, ExpiredAt), "va_timer_expired"; got != want {
		t.Errorf("a timer expiring fires %q, want %q", got, want)
	}
	if got, want := Event(KindCommand, ExpiredAt), "va_timer_command_expired"; got != want {
		t.Errorf("a command expiring fires %q, want %q", got, want)
	}
	if Event(KindAlarm, Started) == Event(KindCommand, Started) {
		t.Error("an alarm and a command fire the same event")
	}
}

func TestATimerIsOverOnlyWhenItIs(t *testing.T) {
	for _, s := range []Status{Running, Snoozed} {
		if s.Over() {
			t.Errorf("%q reports as over", s)
		}
	}
	for _, s := range []Status{Expired, Cancelled} {
		if !s.Over() {
			t.Errorf("%q does not report as over", s)
		}
	}
}

func TestKindsAreKnownAndNothingElseIs(t *testing.T) {
	for _, k := range Kinds {
		if !k.Known() {
			t.Errorf("%q is in Kinds and reports as unknown", k)
		}
	}
	if Kind("stopwatch").Known() {
		t.Error("a kind View Assist does not define reports as known")
	}
}

func TestAnActionIsOursOrItIsNot(t *testing.T) {
	for _, a := range Answerable {
		if !Served(a) {
			t.Errorf("%q is answerable and Served says no", a)
		}
	}
	for _, a := range Elsewhere {
		if Served(a) {
			t.Errorf("%q belongs to the integration and Served says it is ours", a)
		}
	}
	if Served("navigate_somewhere") {
		t.Error("an action that does not exist reports as served")
	}
}

func TestAnEmptyStateIsHowAScreenIsCleared(t *testing.T) {
	if !(State{}).Empty() {
		t.Error("a state with nothing in it is not empty")
	}
	for _, s := range []State{{Title: "Kitchen"}, {Message: "the oven is on"}} {
		if s.Empty() {
			t.Errorf("%+v reports as empty, want either alone to be meaningful", s)
		}
	}
}
