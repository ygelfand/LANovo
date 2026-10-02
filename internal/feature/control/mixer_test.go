package control

import "testing"

// Control names have spaces in them — "AUDIO_REF_EC_UL1 MUX" — so the value cannot be told from
// the name by position. An equals separates them.
func TestAControlNameKeepsItsSpaces(t *testing.T) {
	name, value, setting := either([]string{"AUDIO_REF_EC_UL1", "MUX"})

	if name != "AUDIO_REF_EC_UL1 MUX" {
		t.Errorf("name is %q", name)
	}
	if setting || value != "" {
		t.Errorf("reading was taken as setting %q", value)
	}
}

func TestAnEqualsSeparatesTheValue(t *testing.T) {
	name, value, setting := either([]string{"AUDIO_REF_EC_UL1", "MUX", "=", "QUAT_MI2S_RX"})

	if !setting {
		t.Fatal("an equals was not read as setting")
	}
	if name != "AUDIO_REF_EC_UL1 MUX" {
		t.Errorf("name is %q", name)
	}
	if value != "QUAT_MI2S_RX" {
		t.Errorf("value is %q", value)
	}
}

// A value with a space in it is one value. Some of the card's items have them.
func TestAValueKeepsItsSpaces(t *testing.T) {
	_, value, _ := either([]string{"Some", "Control", "=", "Two", "Words"})

	if value != "Two Words" {
		t.Errorf("value is %q", value)
	}
}

// Setting a control to nothing is a mistake worth reporting rather than a way to clear it: the
// card has no empty item, so this would otherwise be refused with a list of what it does take.
func TestAnEqualsWithNothingAfterItIsStillSetting(t *testing.T) {
	name, value, setting := either([]string{"Control", "="})

	if !setting {
		t.Error("an equals with nothing after it was read as a question")
	}
	if name != "Control" || value != "" {
		t.Errorf("name %q, value %q", name, value)
	}
}
