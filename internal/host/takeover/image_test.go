package takeover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCubeImageMatchesTheCommittedFile(t *testing.T) {
	path := filepath.Join("../../../assets", CubeBoot.File)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", CubeBoot.File, err)
	}
	if err := CubeBoot.Verify(path, data); err != nil {
		t.Error(err)
	}
	if !strings.Contains(CubeBoot.URL, "/"+BootImagesTag+"/") {
		t.Errorf("published at %q, not under the %s tag", CubeBoot.URL, BootImagesTag)
	}
	if base := filepath.Base(CubeBoot.File); !strings.HasSuffix(CubeBoot.URL, "/"+base) {
		t.Errorf("committed as %q, published at %q", base, CubeBoot.URL)
	}
}
