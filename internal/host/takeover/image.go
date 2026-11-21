package takeover

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"
)

const BootImagesTag = "boot-images"

type Image struct {
	SHA256 string
	Size   int64
	File   string
	URL    string
}

var CubeBoot = Image{
	SHA256: "063a06044c7ce2f4f33f0dfed97df874d6518fdbd0304f0e1af5bc6f6f983a92",
	Size:   12834816,
	File:   "boot/lanovo-boot-cube.img",
	URL:    "https://github.com/ygelfand/LANovo/releases/download/" + BootImagesTag + "/lanovo-boot-cube.img",
}

func BootFor(boardName string) (Image, bool) {
	if boardName == "cube" {
		return CubeBoot, true
	}
	return Image{}, false
}

func (i Image) Verify(from string, data []byte) error {
	if int64(len(data)) != i.Size {
		return fmt.Errorf("takeover: %s is %d bytes, want %d", from, len(data), i.Size)
	}
	if !bytes.HasPrefix(data, []byte(bootMagic)) {
		return fmt.Errorf("takeover: %s does not start with %q", from, bootMagic)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != i.SHA256 {
		return fmt.Errorf("takeover: %s hashes to %s, want %s", from, got, i.SHA256)
	}
	return nil
}

const fetchTimeout = 5 * time.Minute

func (i Image) Resolve(ctx context.Context, progress func(float64)) ([]byte, string, error) {
	if i.URL == "" {
		return nil, "", fmt.Errorf(
			"takeover: no boot image is published, give one with --boot-image",
		)
	}
	data, err := i.download(ctx, progress)
	if err != nil {
		return nil, "", err
	}
	if err := i.Verify(i.URL, data); err != nil {
		return nil, "", err
	}
	return data, i.URL, nil
}

func (i Image) download(ctx context.Context, progress func(float64)) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("takeover: fetching %s: %w", i.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("takeover: fetching %s: %s", i.URL, resp.Status)
	}

	buf := make([]byte, 0, i.Size)
	body := io.LimitReader(resp.Body, i.Size+1)
	chunk := make([]byte, 64*1024)
	for {
		n, err := body.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			if progress != nil && i.Size > 0 {
				progress(min(float64(len(buf))/float64(i.Size), 1))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("takeover: fetching %s: %w", i.URL, err)
		}
	}
	return buf, nil
}
