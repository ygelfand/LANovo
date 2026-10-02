package control

import (
	"fmt"
	"os"
	"time"

	"github.com/ygelfand/LANovo/internal/feature/vision"
)

func still(args []string) (string, error) {
	at := "/data/local/tmp/still.jpg"
	if len(args) > 0 && args[0] != "" {
		at = args[0]
	}

	began := time.Now()
	pic, err := vision.Get().Still()
	if err != nil {
		return "", err
	}
	took := time.Since(began)

	if err := os.WriteFile(at, pic, 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("  %-9s %d bytes in %s\n  %-9s %s\n",
		"picture", len(pic), took.Round(time.Millisecond), "wrote", at), nil
}
