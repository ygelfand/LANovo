package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ygelfand/LANovo/internal/update"
)

func main() {
	var (
		m update.Manifest

		from = flag.String("from", "", "where this release's assets can be fetched from")
		arm  = flag.String("arm", "", "the arm build, hashed and measured")
		out  = flag.String("out", "", "where to write the manifest, or stdout")
	)
	flag.StringVar(&m.Version, "version", "", "version as Home Assistant will compare it")
	flag.StringVar(&m.Title, "title", "", "title for Home Assistant's update card")
	flag.StringVar(&m.Notes, "notes", "", "release notes, shown on the card")
	flag.StringVar(&m.ReleaseURL, "release-url", "", "what the card's link points at")
	flag.Parse()

	if err := run(m, *from, map[string]string{"arm": *arm}, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(m update.Manifest, from string, builds map[string]string, out string) error {
	if m.Version == "" || from == "" || builds["arm"] == "" {
		return fmt.Errorf("mkmanifest: -version, -from and -arm are all required")
	}

	m.Binaries = make(map[string]update.Binary, len(builds))
	for arch, path := range builds {
		if path == "" {
			continue
		}
		b, err := measure(path)
		if err != nil {
			return err
		}
		b.URL = from + "/" + filepath.Base(path)
		m.Binaries[arch] = b
	}

	if err := m.Valid(); err != nil {
		return err
	}

	encoded, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')

	if out == "" {
		_, err = os.Stdout.Write(encoded)
		return err
	}
	return os.WriteFile(out, encoded, 0o644)
}

func measure(path string) (update.Binary, error) {
	f, err := os.Open(path)
	if err != nil {
		return update.Binary{}, err
	}
	defer f.Close()

	sum := sha256.New()
	n, err := io.Copy(sum, f)
	if err != nil {
		return update.Binary{}, err
	}
	return update.Binary{SHA256: hex.EncodeToString(sum.Sum(nil)), Size: n}, nil
}
