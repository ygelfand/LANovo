package say_test

import (
	say "github.com/ygelfand/libcountertop/pkg/say"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Each product verifies its literal message references against the shared catalogue.
// Keys used only by the other product are valid entries in that catalogue.
var asks = regexp.MustCompile(`\b(?:say|text)\.[TFN]\("([^"]+)"`)

func TestEveryIdentifierTheCodeAsksForExists(t *testing.T) {
	ids, err := say.Ids()
	if err != nil {
		t.Fatal(err)
	}

	has := map[string]bool{}
	for _, id := range ids {
		has[id] = true
	}

	for _, file := range sources(t) {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range asks.FindAllSubmatch(body, -1) {
			id := string(m[1])

			// A literal ending in a dot is the front of an identifier built at the call site, so
			// what follows cannot be read here. Whoever owns the list checks its own family.
			if strings.HasSuffix(id, ".") {
				continue
			}

			if !has[id] {
				where, _ := filepath.Rel(root(t), file)
				t.Errorf("%s asks for %q, which en.yaml does not have", where, id)
			}
		}
	}

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
