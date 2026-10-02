package say

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The catalogue against the code that asks of it.
//
// Both directions are faults. An identifier the code asks for and the catalogue does not have draws
// as itself, so a row on the panel reads "settings.title" — which is the sort of thing that reaches
// a device rather than a review, because nothing else complains. An identifier the catalogue has
// and nothing asks for is a string a translator will spend time on for no reason.

// asks finds a call to T, F or N with a literal identifier, however the package is named at the
// call site. Written without an example of one, because this file is scanned too.
var asks = regexp.MustCompile(`\b(?:say|text)\.[TFN]\("([^"]+)"`)

// tables finds a settings table being declared. Its rows' messages are keyed under the domain given
// here and built from it, so no literal at the call site names them.
var tables = regexp.MustCompile(`NewTable\("([^"]+)"`)

// kinds is what internal/setting asks of the catalogue for every table, in the order a row needs
// them: the row's own name, its section, one of its options, and a group's summary.
var kinds = []string{".setting.", ".entity.", ".group.", ".option.", ".sums."}

// dynamic is the identifiers resolved without a literal at the call site, which the scan cannot
// see. Each one needs a reason.
var dynamic = map[string]string{
	// Read out of each message file as it loads, to label the row that picks a language.
	"language.name": "Name reads it per language rather than through a localizer",
}

func TestEveryIdentifierTheCodeAsksForExists(t *testing.T) {
	ids, err := Ids()
	if err != nil {
		t.Fatal(err)
	}

	has := map[string]bool{}
	for _, id := range ids {
		has[id] = true
	}

	used := map[string]bool{}
	var families []string

	for _, file := range sources(t) {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range tables.FindAllSubmatch(body, -1) {
			domain := string(m[1])
			for _, kind := range kinds {
				families = append(families, domain+kind)
			}
			used[domain+".disabled"] = true
		}

		for _, m := range asks.FindAllSubmatch(body, -1) {
			id := string(m[1])

			// A literal ending in a dot is the front of an identifier built at the call site, so
			// what follows cannot be read here. Whoever owns the list checks its own family.
			if strings.HasSuffix(id, ".") {
				families = append(families, id)
				continue
			}
			used[id] = true

			if !has[id] {
				where, _ := filepath.Rel(root(t), file)
				t.Errorf("%s asks for %q, which en.yaml does not have", where, id)
			}
		}
	}

	for _, id := range ids {
		if used[id] || dynamic[id] != "" || inFamily(id, families) {
			continue
		}
		t.Errorf("en.yaml has %q and nothing asks for it", id)
	}
}

func inFamily(id string, families []string) bool {
	for _, at := range families {
		if strings.HasPrefix(id, at) {
			return true
		}
	}
	return false
}

// root is the top of the tree, three up from this package.
func root(t *testing.T) string {
	t.Helper()

	at, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	return at
}

// sources is every Go file in the tree, tests included: a test that asks for a string the catalogue
// lost is as broken as the panel doing it.
func sources(t *testing.T) []string {
	t.Helper()

	var out []string
	err := filepath.Walk(root(t), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "bin", "node_modules", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(out) < 100 {
		t.Fatalf("%d Go files found, so the scan is not reaching the tree", len(out))
	}
	return out
}
