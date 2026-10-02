package say

import (
	"strings"
	"testing"
)

func TestEnglishIsThere(t *testing.T) {
	ids, err := Ids()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) < 50 {
		t.Fatalf("%d identifiers, which is fewer than the panel shows", len(ids))
	}

	for _, id := range ids {
		if got := T(id); got == id {
			t.Errorf("%s reads as its own identifier", id)
		}
	}
}

// The identifiers are the naming scheme, and a stray one is how a screen ends up showing a dotted
// string to somebody.
func TestIdentifiersAreDotted(t *testing.T) {
	ids, err := Ids()
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range ids {
		if !strings.Contains(id, ".") {
			t.Errorf("%q has no screen in front of it", id)
		}
		if strings.ToLower(id) != id {
			t.Errorf("%q is not lower case", id)
		}
	}
}

// An identifier nothing defines comes back as itself rather than empty, so it is visible on the
// panel instead of being a row with nothing in it.
func TestAMissingIdentifierIsItsOwnName(t *testing.T) {
	if got := T("nothing.defines.this"); got != "nothing.defines.this" {
		t.Errorf("a missing identifier came back as %q", got)
	}
	if !Missing("nothing.defines.this") {
		t.Error("a missing identifier does not report as missing")
	}
	if Missing("settings.title") {
		t.Error("a defined identifier reports as missing")
	}
}

// The point of the fallback: a translation that covers half the panel shows the other half in
// English, rather than showing identifiers or refusing to load.
func TestAPartialTranslationFallsBackPerString(t *testing.T) {
	if err := Load("pl", "settings.title: Ustawienia\n"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Use(English.String()) })

	if got := Use("pl"); got != "pl" {
		t.Fatalf("asking for pl settled on %q", got)
	}

	if got := T("settings.title"); got != "Ustawienia" {
		t.Errorf("the translated string is %q", got)
	}
	if got := T("settings.display"); got != "Display" {
		t.Errorf("an untranslated string is %q, want the English", got)
	}
}

// A language with no messages at all is English rather than a panel of identifiers.
func TestAnUnknownLanguageIsEnglish(t *testing.T) {
	t.Cleanup(func() { Use(English.String()) })

	for _, want := range []string{"zz", "klingon", "", "!!"} {
		if got := Use(want); got != "en" {
			t.Errorf("asking for %q settled on %q, want en", want, got)
		}
		if got := T("settings.title"); got != "Settings" {
			t.Errorf("after asking for %q the title is %q", want, got)
		}
	}
}

// Untranslated is what a translator reads to know what is left.
func TestUntranslatedReportsWhatIsLeft(t *testing.T) {
	if err := Load("fi", "settings.title: Asetukset\n"); err != nil {
		t.Fatal(err)
	}

	left, err := Untranslated("fi")
	if err != nil {
		t.Fatal(err)
	}

	ids, _ := Ids()
	if len(left) != len(ids)-1 {
		t.Errorf("%d identifiers left of %d, want all but the one translated", len(left), len(ids))
	}
	for _, id := range left {
		if id == "settings.title" {
			t.Error("the one translated string is reported as untranslated")
		}
	}
}

// English is offered first, because it is the one that is complete.
func TestEnglishIsOfferedFirst(t *testing.T) {
	if got := Languages(); len(got) == 0 || got[0] != "en" {
		t.Errorf("the languages are %v", got)
	}
}
