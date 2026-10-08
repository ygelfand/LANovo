package display

import "testing"

func noop(*Panel) error { return nil }

func TestTopmostTakesThePriority(t *testing.T) {
	d := NewDriver("/dev/null")

	under := d.Claim(PriorityDashboard)
	under.Show(noop)
	over := d.Claim(PriorityAlert)
	over.Show(noop)

	if got := d.topmost(); got != over {
		t.Error("the lower claim took the panel")
	}
}

func TestTopmostPrefersTheNewestOfEqualClaims(t *testing.T) {
	d := NewDriver("/dev/null")

	first := d.Claim(PriorityNotice)
	first.Show(noop)
	second := d.Claim(PriorityNotice)
	second.Show(noop)

	if got := d.topmost(); got != second {
		t.Error("an equal claim made later did not take the panel")
	}
}

func TestClaimWithNothingToDrawDoesNotBlock(t *testing.T) {
	d := NewDriver("/dev/null")

	under := d.Claim(PriorityDashboard)
	under.Show(noop)
	d.Claim(PriorityAlert)

	if got := d.topmost(); got != under {
		t.Error("an empty claim took the panel from the one drawing under it")
	}
}

func TestClearFallsThroughToWhatIsUnder(t *testing.T) {
	d := NewDriver("/dev/null")

	under := d.Claim(PriorityDashboard)
	under.Show(noop)
	over := d.Claim(PriorityAlert)
	over.Show(noop)

	over.Clear()

	if got := d.topmost(); got != under {
		t.Error("clearing the top claim did not give the panel back")
	}
}

func TestReleaseFallsThroughToWhatIsUnder(t *testing.T) {
	d := NewDriver("/dev/null")

	under := d.Claim(PriorityDashboard)
	under.Show(noop)
	over := d.Claim(PriorityAlert)
	over.Show(noop)

	over.Release()

	if got := d.topmost(); got != under {
		t.Error("releasing the top claim did not give the panel back")
	}
}

func TestTopmostWithNothingDrawing(t *testing.T) {
	d := NewDriver("/dev/null")
	d.Claim(PriorityBoot)

	if got := d.topmost(); got != nil {
		t.Error("a claim with nothing to draw was chosen to render")
	}
}

func TestSetOrientationDoesNotTouchThePanel(t *testing.T) {
	d := NewDriver("/dev/null")

	claim := d.Claim(PriorityDashboard)
	claim.Show(noop)

	d.SetOrientation(Rotate180)

	if got := d.Orientation(); got != Rotate180 {
		t.Errorf("the driver is at %v, want %v", got, Rotate180)
	}

	if err := d.render(); err != nil {
		t.Errorf("render with no panel: %v", err)
	}
}

func TestStartupBeforeThePanelIsOpen(t *testing.T) {
	d := NewDriver("/dev/null")

	got := d.Startup()
	if got.Done {
		t.Error("the display says it is up with no panel open")
	}
	if got.Doing == "" {
		t.Error("the display says nothing about what it is waiting for")
	}
}

func TestStackDrawsTheCoveringClaimAndEverythingAbove(t *testing.T) {
	d := NewDriver("/dev/null")

	under := d.Claim(PriorityDashboard)
	under.Show(noop)
	setup := d.Claim(PrioritySetup)
	setup.Show(noop)
	over := d.Overlay(PriorityNotice)
	over.Show(noop)

	stack, clear := d.stack()
	if clear {
		t.Error("something covers the panel, so it does not need painting out")
	}
	if len(stack) != 2 || stack[0] != setup || stack[1] != over {
		t.Fatalf("stack is %d deep, want the setup screen then the overlay", len(stack))
	}
}

func TestStackClearsWhenNothingCovers(t *testing.T) {
	d := NewDriver("/dev/null")

	over := d.Overlay(PriorityNotice)
	over.Show(noop)

	stack, clear := d.stack()
	if !clear {
		t.Error("an overlay with nothing under it did not ask for the panel to be cleared")
	}
	if len(stack) != 1 || stack[0] != over {
		t.Errorf("stack is %d deep, want just the overlay", len(stack))
	}
}

func TestStackStartsAtTheHighestCoveringClaim(t *testing.T) {
	d := NewDriver("/dev/null")

	over := d.Overlay(PriorityNotice)
	over.Show(noop)
	boot := d.Claim(PriorityBoot)
	boot.Show(noop)

	stack, _ := d.stack()
	if len(stack) != 1 || stack[0] != boot {
		t.Errorf("stack is %d deep, want only the boot screen", len(stack))
	}
}

func TestStackSkipsClaimsWithNothingToDraw(t *testing.T) {
	d := NewDriver("/dev/null")

	under := d.Claim(PriorityDashboard)
	under.Show(noop)
	d.Overlay(PriorityNotice)

	stack, _ := d.stack()
	if len(stack) != 1 || stack[0] != under {
		t.Errorf("stack is %d deep, want only the claim that draws", len(stack))
	}
}

func TestCoveredAgreesWithTopmost(t *testing.T) {
	d := NewDriver("/dev/null")

	boot := d.Claim(PriorityBoot)
	boot.Show(noop)
	d.Claim(PriorityDashboard)

	if d.Covered(PriorityBoot) {
		t.Fatal("Covered says something below is drawing when nothing is")
	}

	under := d.Claim(PriorityDashboard)
	under.Show(noop)

	if !d.Covered(PriorityBoot) {
		t.Error("Covered missed a claim that is drawing below")
	}
	if got := d.topmost(); got != boot {
		t.Error("boot outranks the dashboard and should still hold the panel")
	}
}

func TestDamageIsAllOrNothing(t *testing.T) {
	d := NewDriver("")

	under := d.Claim(PriorityDashboard)
	over := d.Overlay(PriorityNotice)

	under.Show(func(*Panel) error { return nil })
	over.Show(func(*Panel) error { return nil })

	stack, _ := d.stack()
	if got, _ := damageOf(stack); got.W != 0 || got.H != 0 {
		t.Errorf("damage = %+v with nobody having named one, want the whole screen", got)
	}

	under.Refresh(Rect{X: 0, Y: 0, W: 10, H: 10})
	over.Refresh(Rect{X: 100, Y: 100, W: 10, H: 10})

	stack, _ = d.stack()
	want := Rect{X: 0, Y: 0, W: 110, H: 110}
	if got, _ := damageOf(stack); got != want {
		t.Errorf("damage = %+v, want %+v covering both", got, want)
	}

	over.Show(func(*Panel) error { return nil })

	stack, _ = d.stack()
	if got, _ := damageOf(stack); got.W != 0 || got.H != 0 {
		t.Errorf("damage = %+v after a full redraw was asked for, want the whole screen", got)
	}
}

func TestUnionCarriesThePreviousFrame(t *testing.T) {
	first := Rect{X: 0, Y: 0, W: 10, H: 10}
	second := Rect{X: 90, Y: 90, W: 10, H: 10}

	got := union(second, first)
	want := Rect{X: 0, Y: 0, W: 100, H: 100}
	if got != want {
		t.Errorf("union = %+v, want %+v", got, want)
	}

	if got := union(first, Rect{}); got != (Rect{}) {
		t.Errorf("union with the whole screen = %+v, want the whole screen", got)
	}
}

func TestTwoRepaintsBetweenFramesAreBothPainted(t *testing.T) {
	d := NewDriver("")
	c := d.Claim(PriorityDashboard)
	c.Show(noop)

	stack, _ := d.stack()
	damageOf(stack)

	c.ShowIn(Rect{X: 0, Y: 0, W: 10, H: 10}, noop)
	c.ShowIn(Rect{X: 100, Y: 100, W: 10, H: 10}, noop)

	want := Rect{X: 0, Y: 0, W: 110, H: 110}
	if got, _ := damageOf(stack); got != want {
		t.Errorf("damage = %+v, want %+v covering both repaints", got, want)
	}
}

func TestARepaintAfterTheDamageIsTakenWaitsForTheNextFrame(t *testing.T) {
	d := NewDriver("")
	c := d.Claim(PriorityDashboard)
	c.Show(noop)

	stack, _ := d.stack()
	damageOf(stack)
	if _, any := damageOf(stack); any {
		t.Fatal("damage taken once came back a second time")
	}

	late := Rect{X: 5, Y: 5, W: 10, H: 10}
	c.Refresh(late)
	if got, any := damageOf(stack); !any || got != late {
		t.Errorf("a repaint after the take gave %+v, %v; want %+v", got, any, late)
	}
}

func TestOnlyACoveringClaimWithSomethingToDrawCoversTheDashboard(t *testing.T) {
	d := NewDriver("/dev/null")
	d.Claim(PriorityDashboard).Show(noop)
	if !d.Uncovered(PriorityDashboard) {
		t.Fatal("the dashboard alone reads as covered")
	}

	d.Overlay(PriorityAlert).Show(noop)
	if !d.Uncovered(PriorityDashboard) {
		t.Error("an overlay covered the dashboard")
	}

	card := d.Claim(PriorityNotice)
	if !d.Uncovered(PriorityDashboard) {
		t.Error("an empty claim covered the dashboard")
	}
	card.Show(noop)
	if d.Uncovered(PriorityDashboard) {
		t.Error("a drawing card did not cover the dashboard")
	}
	card.Clear()
	if !d.Uncovered(PriorityDashboard) {
		t.Error("a cleared card still covers the dashboard")
	}
}

func TestClearingAClaimRepaintsWhereItWas(t *testing.T) {
	d := NewDriver("/dev/null")
	under := d.Claim(PriorityDashboard)
	under.Show(noop)
	strip := d.Overlay(PrioritySetup)
	strip.Show(noop)
	stack, _ := d.stack()
	damageOf(stack)
	d.forced = false

	strip.Clear()
	if !d.forced {
		t.Error("clearing an overlay left what it covered to the buffers")
	}
}
