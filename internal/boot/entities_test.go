package boot

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/component"
)

// esphome derives an entity's Key as fnv1(ObjectID).
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

		if first, clash := keys[e.Key()]; clash && first != id {
			t.Errorf("%q and %q hash to the same key %d", first, id, e.Key())
		}
		keys[e.Key()] = id
	}

	t.Logf("%d entities, all distinct", len(entities))
}

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

	if total := len(component.Default().Entities()); named < total {
		t.Errorf("only %d of %d entities had a name that could be read, so the rest went unchecked",
			named, total)
	}
}

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
