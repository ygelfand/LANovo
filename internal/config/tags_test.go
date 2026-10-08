package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEverySettingIsPersisted(t *testing.T) {
	skip := map[string]bool{"Config.Device": true}

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
			switch name {
			case "-":
				t.Errorf("%s is tagged away, so setting it does not survive a restart", where)
				continue
			case "":
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

	if visited < 40 {
		t.Errorf("only %d fields were checked, so the walk is not reaching the sections", visited)
	}
	if !seenWakeWord {
		t.Error(
			"the walk did not reach inside Wake.Words, so a list's fields are not being checked",
		)
	}
}

func TestDefaultsSurviveBeingWrittenAndReadBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	st, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

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
