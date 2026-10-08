package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"
)

// Version has to stay rankable by Home Assistant's AwesomeVersion.
type Manifest struct {
	Version    string            `json:"version"`
	Binaries   map[string]Binary `json:"binaries"`
	Title      string            `json:"title,omitempty"`
	Notes      string            `json:"notes,omitempty"`
	ReleaseURL string            `json:"release_url,omitempty"`
}

type Binary struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

var arch = runtime.GOARCH

const (
	manifestTimeout = 10 * time.Second
	maxManifest     = 64 << 10
)

func Fetch(ctx context.Context, c Channel) (Manifest, error) {
	var m Manifest
	url := c.URL()

	ctx, cancel := context.WithTimeout(ctx, manifestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return m, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return m, fmt.Errorf("update: fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return m, fmt.Errorf("update: fetching %s: %s", url, resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxManifest)).Decode(&m); err != nil {
		return m, fmt.Errorf("update: reading the manifest at %s: %w", url, err)
	}
	return m, m.Valid()
}

func (m Manifest) Valid() error {
	if m.Version == "" {
		return errors.New("update: the manifest names no version")
	}
	if len(m.Binaries) == 0 {
		return fmt.Errorf("update: the manifest for %s carries no binaries", m.Version)
	}
	for a, b := range m.Binaries {
		switch {
		case b.URL == "":
			return fmt.Errorf("update: the %s binary for %s has no url", a, m.Version)
		case len(b.SHA256) != 64:
			return fmt.Errorf("update: the %s binary for %s has no usable sha256", a, m.Version)
		case b.Size <= 0:
			return fmt.Errorf("update: the %s binary for %s gives no size", a, m.Version)
		}
	}
	return nil
}

func (m Manifest) For(arch string) (Binary, error) {
	if b, ok := m.Binaries[arch]; ok {
		return b, nil
	}
	return Binary{}, fmt.Errorf("update: %s carries no %s build", m.Version, arch)
}
