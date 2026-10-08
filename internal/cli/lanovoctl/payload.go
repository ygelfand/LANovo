package lanovoctl

import (
	"fmt"
	"os"

	"github.com/ygelfand/LANovo/internal/host/assets"
)

func lanovod(path string) ([]byte, string, error) {
	if path != "" {
		data, err := os.ReadFile(path)
		return data, path, err
	}
	if !assets.Embedded() {
		return nil, "", fmt.Errorf(
			"this build ships no lanovod: build with `make dist`, or pass --binary",
		)
	}
	return assets.Lanovod(), "shipped with lanovoctl", nil
}
