package shell

import (
	"testing"
	"time"
)

type view struct {
	covers bool
	asleep bool
	wakes  bool
	wait   time.Duration
}

func (v *view) Covers() bool { return v.covers }
func (v *view) Asleep() bool { return v.asleep }
func (v *view) Wakes() bool  { return v.wakes }
func (v *view) Timeout() time.Duration {
	if v.wait == 0 {
		return SettingsTimeout
	}
	return v.wait
}

func TestReleasingAHoldTakesAwayThatViewAndNoOther(t *testing.T) {
	s := &Shell{}
	player, page := &view{covers: true}, &view{covers: true}
	h := s.Hold(player)
	s.Push(page)
	h.Release()
	if got := s.Views(); len(got) != 1 || got[0] != page {
		t.Errorf("left %v, want the page", got)
	}
	if h.Held() {
		t.Error("still held after release")
	}
}

func TestIdlingKeepsWhatIsHeld(t *testing.T) {
	s := &Shell{}
	player, page := &view{covers: true}, &view{covers: true}
	s.Hold(player)
	s.Push(page)
	s.idled()
	if got := s.Views(); len(got) != 1 || got[0] != player {
		t.Errorf("left %v, want the held player", got)
	}
}

func TestRemovingAViewDropsItsHold(t *testing.T) {
	s := &Shell{}
	idle := &view{covers: true}
	s.Hold(idle)
	s.Remove(idle)
	if s.Holding(idle) || s.Open() {
		t.Error("a removed view is still held or the shell is still open")
	}
}

func TestASleepingScreenRefusesWhatDoesNotWakeIt(t *testing.T) {
	s := &Shell{}
	idle := &view{covers: true, asleep: true}
	s.Hold(idle)
	s.Push(&view{covers: true})
	if s.Top() != View(idle) {
		t.Error("a covering view replaced the sleeping screen")
	}
	video := &view{covers: true, wakes: true}
	s.Push(video)
	if s.Top() != View(video) {
		t.Error("a view that wakes did not come up over the sleeping screen")
	}
}

func TestVisibleIsTheCoveringViewAndWhatIsAboveIt(t *testing.T) {
	s := &Shell{}
	under, page, card := &view{covers: true}, &view{covers: true}, &view{}
	s.Push(under)
	s.Push(page)
	s.Push(card)
	if s.Visible(under) || !s.Visible(page) || !s.Visible(card) {
		t.Errorf("visible: under %v page %v card %v", s.Visible(under), s.Visible(page), s.Visible(card))
	}
}

func TestTheLongestTimeoutInTheStackWins(t *testing.T) {
	s := &Shell{}
	s.Push(&view{wait: DockTimeout})
	s.Push(&view{covers: true})
	if got := s.timeout(); got != SettingsTimeout {
		t.Errorf("timeout %v, want %v", got, SettingsTimeout)
	}
}
