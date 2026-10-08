package config

import (
	"testing"
	"time"
)

func TestAFreshDeviceIdlesIntoTheClockItAlreadyShows(t *testing.T) {
	c, idle := Defaults().Clock, Defaults().Idle

	if idle.Face != c.Face || idle.Position != c.Position || idle.Size != c.Size ||
		idle.Align != AlignCenter {
		t.Errorf("idle %+v differs from the clock %+v", idle, c)
	}
	if idle.First.On() || idle.Second.On() {
		t.Error("a fresh device idles with a visual")
	}
	if idle.After.After() <= 0 {
		t.Errorf("a fresh device has no wait, after %v", idle.After)
	}
}

func TestNeverIsNoWaitAtAll(t *testing.T) {
	if got := DelayNever.After(); got != 0 {
		t.Errorf("never is %v, want zero", got)
	}

	for _, d := range Delays() {
		if d == DelayNever {
			continue
		}
		if d.After() <= 0 {
			t.Errorf("%v waits %v", d, d.After())
		}
	}
}

func TestTheDelaysAreInOrder(t *testing.T) {
	delays := Delays()

	if got := delays[len(delays)-1]; got != DelayNever {
		t.Errorf("the last choice is %v, want Never", got)
	}

	var last time.Duration
	for _, d := range delays[:len(delays)-1] {
		if d.After() <= last {
			t.Errorf("%v comes after %v", d, last)
		}
		last = d.After()
	}
}

func TestEveryDelayRoundTripsThroughItsLabel(t *testing.T) {
	seen := map[string]Delay{}

	for _, d := range Delays() {
		label := d.Label()
		if had, ok := seen[label]; ok {
			t.Errorf("%v and %v are both shown as %q", had, d, label)
		}
		seen[label] = d

		got, ok := ByLabel(Delays(), label)
		if !ok || got != d {
			t.Errorf("%q resolved to %v %v, want %v", label, got, ok, d)
		}
	}
}

func TestTheIdleFaceIsItsOwnSetting(t *testing.T) {
	fresh(t)

	if err := Set().Idle().Face(FaceNone); err != nil {
		t.Fatal(err)
	}
	if err := Set().Clock().Face(FaceAnalogSeconds); err != nil {
		t.Fatal(err)
	}

	if c, idle := Get().Clock, Get().Idle; c.Face != FaceAnalogSeconds || idle.Face != FaceNone {
		t.Errorf("clock is %v and idle is %v, want analog-seconds and none", c.Face, idle.Face)
	}
}

func TestEachIdleVisualIsItsOwnSetting(t *testing.T) {
	fresh(t)

	if err := Set().Idle().Visual(1, IdleVisual{Kind: "orb", Source: SourceMic}); err != nil {
		t.Fatal(err)
	}
	idle := Get().Idle
	if idle.First.On() || idle.Second != (IdleVisual{Kind: "orb", Source: SourceMic}) {
		t.Errorf("first %+v second %+v", idle.First, idle.Second)
	}
}
