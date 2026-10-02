package viewassist

import "testing"

func TestParsingTheThreeStatusItemForms(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want StatusItem
	}{
		{"view:clock|mdi:clock", StatusItem{StatusView, "clock", "mdi:clock"}},
		{"entity:binary_sensor.front_door|mdi:door",
			StatusItem{StatusEntity, "binary_sensor.front_door", "mdi:door"}},
		{"service:script.goodnight|mdi:weather-night",
			StatusItem{StatusService, "script.goodnight", "mdi:weather-night"}},
	} {
		got, ok := ParseStatus(tc.in)
		if !ok {
			t.Errorf("%q was not read as a status item", tc.in)
			continue
		}
		if got != tc.want {
			t.Errorf("%q read as %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

// The icon is the optional half. An item without one is still an item; what it draws is the
// screen's business.
func TestAStatusItemWithNoIcon(t *testing.T) {
	got, ok := ParseStatus("entity:light.kitchen")
	if !ok {
		t.Fatal("an item with no icon was not read as one")
	}
	if got.Icon != "" {
		t.Errorf("it came back with icon %q, want none", got.Icon)
	}
	if got.Target != "light.kitchen" {
		t.Errorf("its target is %q", got.Target)
	}
}

// An mdi name has a colon in it, so splitting the kind off has to stop at the first one.
func TestTheIconKeepsItsOwnColon(t *testing.T) {
	got, _ := ParseStatus("view:weather|mdi:weather-partly-cloudy")
	if got.Icon != "mdi:weather-partly-cloudy" {
		t.Errorf("the icon read as %q", got.Icon)
	}
}

func TestWhatIsNotAStatusItem(t *testing.T) {
	for _, in := range []string{
		"",
		"clock",                      // no kind
		"widget:clock|mdi:clock",     // not one of the three
		"view:|mdi:clock",            // no target
		"view:clock|mdi:clock|extra", // a second bar belongs to no form
	} {
		if got, ok := ParseStatus(in); ok {
			t.Errorf("%q was read as a status item: %+v", in, got)
		}
	}
}

// What is sent on should be what arrived, so an item this device passes through unchanged is
// unchanged.
func TestAStatusItemWritesBackAsItWasRead(t *testing.T) {
	for _, in := range []string{
		"view:clock|mdi:clock",
		"entity:light.kitchen",
		"service:script.goodnight|mdi:weather-night",
	} {
		got, ok := ParseStatus(in)
		if !ok {
			t.Fatalf("%q was not read as a status item", in)
		}
		if got.String() != in {
			t.Errorf("%q written back as %q", in, got.String())
		}
	}
}

// A view item usually names a view, and is allowed not to: their status icons carry external paths
// as well, which is the same reason Navigated answers with whether it found one.
func TestAViewStatusItemNamesAViewOrSaysItDoesNot(t *testing.T) {
	item, _ := ParseStatus("view:/viewassist/music|mdi:music")
	if v, ok := item.View(); !ok || v != Music {
		t.Errorf("the item names %q (%v), want the music view", v, ok)
	}

	outside, _ := ParseStatus("view:/lovelace/garage|mdi:garage")
	if v, ok := outside.View(); ok {
		t.Errorf("a path outside the dashboard came back as the %q view", v)
	}

	entity, _ := ParseStatus("entity:light.kitchen|mdi:lamp")
	if _, ok := entity.View(); ok {
		t.Error("an entity item answered as a view")
	}
}
