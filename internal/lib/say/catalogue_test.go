package say_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	say "github.com/ygelfand/libcountertop/pkg/say"
)

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

func root(t *testing.T) string {
	t.Helper()

	at, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	return at
}

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
