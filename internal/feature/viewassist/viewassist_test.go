package viewassist

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/lib/viewassist"
)

func TestNavigatingTakesTheView(t *testing.T) {
	s := &Satellite{view: viewassist.Clock}

	if err := s.Navigate("/viewassist/weather"); err != nil {
		t.Fatalf("navigating to the weather view: %v", err)
	}
	if got := s.View(); got != viewassist.Weather {
		t.Errorf("on %q, want %q", got, viewassist.Weather)
	}
}

// Their status icons carry external paths alongside view names, so a path that names no view has to
// be refused. A satellite that silently accepted one would look like it had obeyed.
func TestAPathThatNamesNoViewIsRefused(t *testing.T) {
	s := &Satellite{view: viewassist.Clock}

	for _, path := range []string{"", "/lovelace/0", "https://example.com"} {
		if err := s.Navigate(path); err == nil {
			t.Errorf("%q was accepted, want it refused", path)
		}
	}
	if got := s.View(); got != viewassist.Clock {
		t.Errorf("a refused navigation moved the view to %q", got)
	}
}

// A webpage is markup and there is no browser here, which is the whole reason the rest are drawn
// natively. Refusing it says so where an automation can see it.
func TestTheWebpageViewIsRefused(t *testing.T) {
	s := &Satellite{view: viewassist.Clock}

	if err := s.Navigate(viewassist.Webpage.Path()); err == nil {
		t.Error("the webpage view was accepted")
	}
	if got := s.View(); got != viewassist.Clock {
		t.Errorf("a refused navigation moved the view to %q", got)
	}
}

// Every view this device can draw has to be reachable, or an automation written against the
// contract hits one the satellite silently will not take.
func TestEveryDrawableViewIsReachable(t *testing.T) {
	s := &Satellite{view: viewassist.Clock}

	for _, v := range viewassist.Views {
		if !v.Drawable() {
			continue
		}
		if err := s.Navigate(v.Path()); err != nil {
			t.Errorf("%q: %v", v, err)
			continue
		}
		if got := s.View(); got != v {
			t.Errorf("navigating to %q left the view at %q", v, got)
		}
	}
}

// The state is readable back, which is what a view will draw from once there is one.
func TestTheStateIsKept(t *testing.T) {
	s := &Satellite{}
	said := viewassist.State{Title: "Kitchen", Message: "the oven is on"}

	s.SetState(said)

	if got := s.State(); got != said {
		t.Errorf("state %+v, want %+v", got, said)
	}
}
