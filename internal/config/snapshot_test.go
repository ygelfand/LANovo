package config

import "testing"

func TestSnapshotOwnsEveryMutableSettingsCollection(t *testing.T) {
	s, err := Load(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Update(func(c *Config) {
		c.Wake.Words = []WakeWord{{ID: "word"}}
		c.Cast.YouTube.Skip = []string{"sponsor"}
		c.Camera.Settings = map[string]string{"scene": "auto"}
		c.Home.Control = map[string]bool{"x": true}
		c.Home.Group = map[string]int{"x": 2}
		c.Home.Picks = map[string]HomePick{
			"x": {Labels: []string{"label"}, Areas: []string{"area"}, Entities: []string{"entity"}},
		}
	}); err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	got.Wake.Words[0].ID = "changed"
	got.Cast.YouTube.Skip[0] = "changed"
	got.Camera.Settings["scene"] = "changed"
	got.Home.Control["x"] = false
	got.Home.Group["x"] = 4
	pick := got.Home.Picks["x"]
	pick.Labels[0] = "changed"
	pick.Areas[0] = "changed"
	pick.Entities[0] = "changed"
	got.Home.Picks["x"] = pick
	actual := s.Get()
	if actual.Wake.Words[0].ID != "word" || actual.Cast.YouTube.Skip[0] != "sponsor" ||
		actual.Camera.Settings["scene"] != "auto" ||
		!actual.Home.Control["x"] ||
		actual.Home.Group["x"] != 2 {
		t.Fatal("snapshot mutated store", actual)
	}
	pick = actual.Home.Picks["x"]
	if pick.Labels[0] != "label" || pick.Areas[0] != "area" || pick.Entities[0] != "entity" {
		t.Fatal("nested snapshot mutated store", pick)
	}
}
