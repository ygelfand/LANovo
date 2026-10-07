package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Every setting is written to the file, and a setting that is not says nothing about itself.
//
// The failure this catches: a field added to a section without a json tag, or with one that
// collides with a neighbour. Neither is a compile error and neither shows up until somebody sets
// the thing, restarts, and finds it back where it was. Reflection rather than a list, so a section
// added later is covered by existing rather than by being remembered.
func TestEverySettingIsPersisted(t *testing.T) {
	// Device is what the process was told rather than what anyone chose, and is deliberately not
	// written. TestDeviceIsNotPersisted is the other half of that.
	skip := map[string]bool{"Config.Device": true}

	// A section can hold a list of things — Wake.Words is a []WakeWord — and a field missing its
	// tag inside one of those is lost exactly the same way, so the element type is walked too.
	var inside func(reflect.Type) reflect.Type
	inside = func(at reflect.Type) reflect.Type {
		switch at.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
			return inside(at.Elem())
		}
		return at
	}

	var visited int
	var seenWakeWord bool

	var walk func(at reflect.Type, path string)
	walk = func(at reflect.Type, path string) {
		at = inside(at)
		if at.Kind() != reflect.Struct {
			return
		}
		if at.Name() == "WakeWord" {
			seenWakeWord = true
		}

		seen := map[string]string{}

		for i := range at.NumField() {
			f := at.Field(i)
			if !f.IsExported() {
				continue
			}

			where := path + "." + f.Name
			tag, ok := f.Tag.Lookup("json")

			switch {
			case skip[where]:
				continue
			case !ok:
				t.Errorf(
					"%s has no json tag, so it is written under its Go name by accident",
					where,
				)
				continue
			}

			name, _, _ := strings.Cut(tag, ",")
			switch {
			case name == "-":
				t.Errorf("%s is tagged away, so setting it does not survive a restart", where)
				continue
			case name == "":
				t.Errorf("%s has an empty json tag", where)
				continue
			}

			if first, clash := seen[name]; clash {
				t.Errorf("%s and %s are both written as %q, so one of them is lost",
					first, where, name)
			}
			seen[name] = where
			visited++

			walk(f.Type, where)
		}
	}

	walk(reflect.TypeOf(Config{}), "Config")

	// A walk that reached nothing would pass everything above it. This is a floor, not a count to
	// keep up to date: it only has to be high enough that a broken walk trips it.
	if visited < 40 {
		t.Errorf("only %d fields were checked, so the walk is not reaching the sections", visited)
	}
	if !seenWakeWord {
		t.Error(
			"the walk did not reach inside Wake.Words, so a list's fields are not being checked",
		)
	}
}

// A device nobody has touched reads back exactly as it started.
//
// Not the same as the above: a field can be tagged and still not survive, if its type does not
// marshal to something that unmarshals back. Defaults are what almost every device is running, so
// they are the round trip worth being sure of.
func TestDefaultsSurviveBeingWrittenAndReadBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	st, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Something has to be written for the file to exist at all.
	if err := st.Set().Screen().Backlight(DefaultBacklight); err != nil {
		t.Fatalf("Backlight: %v", err)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}

	want, got := Defaults(), again.Get()
	want.Device, got.Device = Device{}, Device{}

	if !reflect.DeepEqual(want, got) {
		t.Errorf(
			"a fresh device came back different after a write and a reload:\n got %+v\nwant %+v",
			got,
			want,
		)
	}
}
