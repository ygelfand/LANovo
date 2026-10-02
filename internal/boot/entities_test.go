package boot

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/component"
)

// Every entity Home Assistant is shown has its own identity.
//
// esphome derives an entity's key from its object id — Key is fnv1(ObjectID) — so two entities
// sharing an id share a key, whichever device they are put on. Home Assistant would then have two
// things answering to one address, and what it does with that is not something to find out from a
// device in a room.
//
// This package imports component/all, so what it sees is every component the binary has, not a list
// somebody has to keep up to date.
func TestEveryEntityHasItsOwnObjectID(t *testing.T) {
	entities := component.Default().Entities()
	if len(entities) == 0 {
		t.Fatal("no entities are registered, so this is checking nothing")
	}

	seen := map[string]bool{}
	keys := map[uint32]string{}

	for _, e := range entities {
		id := e.Object()

		if id == "" {
			t.Errorf("an entity of type %T has no object id", e)
			continue
		}
		if seen[id] {
			t.Errorf("%q is the object id of more than one entity", id)
		}
		seen[id] = true

		// Different ids that hash the same are the same collision by another route, and the hash is
		// a 32 bit fnv1 over a few dozen short strings, so it is worth being sure rather than
		// assuming.
		if first, clash := keys[e.Key()]; clash && first != id {
			t.Errorf("%q and %q hash to the same key %d", first, id, e.Key())
		}
		keys[e.Key()] = id
	}

	t.Logf("%d entities, all distinct", len(entities))
}

// An object id is part of the entity id Home Assistant files the thing under, so it is written the
// way one is: lower case, digits and underscores.
//
// A capital or a space in one is not rejected anywhere — it just produces something awkward in
// Home Assistant that is then awkward to change, because changing an object id makes Home Assistant
// treat it as a new entity and the old one lingers.
func TestEveryObjectIDIsWrittenLikeAnEntityID(t *testing.T) {
	for _, e := range component.Default().Entities() {
		id := e.Object()

		for _, r := range id {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
			if !ok {
				t.Errorf("%q has %q in it, which does not belong in an object id", id, r)
				break
			}
		}

		if strings.HasPrefix(id, "_") || strings.HasSuffix(id, "_") {
			t.Errorf("%q starts or ends with an underscore", id)
		}
	}
}

// Every entity says what it is. An unnamed one is a blank row in Home Assistant.
//
// Read by reflection because the Entity interface deliberately exposes only Object and Key — the
// rest is the library's business. The embedded Base is where the name lives.
func TestEveryEntityIsNamed(t *testing.T) {
	var named int

	for _, e := range component.Default().Entities() {
		v := reflect.Indirect(reflect.ValueOf(e))
		if v.Kind() != reflect.Struct {
			continue
		}

		base := v.FieldByName("Base")
		if !base.IsValid() {
			continue
		}
		name := base.FieldByName("Name")
		if !name.IsValid() || name.Kind() != reflect.String {
			continue
		}
		named++

		if name.String() == "" {
			t.Errorf("%q has no name", e.Object())
		}
	}

	// Most of them, not one: reading a single name and passing would look the same as working.
	if total := len(component.Default().Entities()); named < total {
		t.Errorf("only %d of %d entities had a name that could be read, so the rest went unchecked",
			named, total)
	}
}

// An action is called by name, so two of them answering to one name is the same fault a step up.
func TestEveryActionHasItsOwnName(t *testing.T) {
	actions := component.Default().Actions()
	if len(actions) == 0 {
		t.Fatal("no actions are registered, so this is checking nothing")
	}

	seen := map[string]bool{}
	for _, a := range actions {
		if a.Name == "" {
			t.Error("an action has no name")
			continue
		}
		if seen[a.Name] {
			t.Errorf("%q is the name of more than one action", a.Name)
		}
		seen[a.Name] = true
	}

	t.Logf("%d actions, all distinct", len(actions))
}
