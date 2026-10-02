// Package immich reads pictures from an Immich server (api.immich.app).
package immich

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxPicture = 32 << 20

type Client struct {
	Base string
	Key  string
	HTTP *http.Client
}

type Album struct {
	ID   string `json:"id"`
	Name string `json:"albumName"`
}

type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Asset struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func New(base, key string) *Client {
	return &Client{Base: strings.TrimRight(base, "/"), Key: key, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) Albums(ctx context.Context) ([]Album, error) {
	var out []Album
	return out, c.call(ctx, http.MethodGet, "/api/albums", nil, &out)
}

func (c *Client) Tags(ctx context.Context) ([]Tag, error) {
	var out []Tag
	return out, c.call(ctx, http.MethodGet, "/api/tags", nil, &out)
}

func (c *Client) Random(ctx context.Context, albums, tags []string, n int) ([]Asset, error) {
	body := map[string]any{"type": "IMAGE", "size": n}
	if len(albums) > 0 {
		body["albumIds"] = albums
	}
	if len(tags) > 0 {
		body["tagIds"] = tags
	}
	var out []Asset
	return out, c.call(ctx, http.MethodPost, "/api/search/random", body, &out)
}

func (c *Client) Preview(ctx context.Context, id string) ([]byte, error) {
	req, err := c.request(ctx, http.MethodGet, "/api/assets/"+url.PathEscape(id)+"/thumbnail?size=preview", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/*")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("immich: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("immich: preview of %s: %s", id, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxPicture+1))
	if err != nil {
		return nil, fmt.Errorf("immich: preview of %s: %w", id, err)
	}
	if len(b) > maxPicture {
		return nil, fmt.Errorf("immich: preview of %s is over %d bytes", id, maxPicture)
	}
	return b, nil
}

// Resolve turns album and tag names into ids; a tag matches its name or its full "parent/child" value.
func (c *Client) Resolve(ctx context.Context, albumNames, tagNames []string) (albums, tags []string, err error) {
	if len(albumNames) > 0 {
		all, err := c.Albums(ctx)
		if err != nil {
			return nil, nil, err
		}
		for _, want := range albumNames {
			id := ""
			for _, a := range all {
				if strings.EqualFold(a.Name, want) {
					id = a.ID
					break
				}
			}
			if id == "" {
				return nil, nil, fmt.Errorf("immich: no album named %q", want)
			}
			albums = append(albums, id)
		}
	}
	if len(tagNames) > 0 {
		all, err := c.Tags(ctx)
		if err != nil {
			return nil, nil, err
		}
		for _, want := range tagNames {
			id := ""
			for _, t := range all {
				if strings.EqualFold(t.Value, want) || strings.EqualFold(t.Name, want) {
					id = t.ID
					break
				}
			}
			if id == "" {
				return nil, nil, fmt.Errorf("immich: no tag named %q", want)
			}
			tags = append(tags, id)
		}
	}
	return albums, tags, nil
}

func (c *Client) call(ctx context.Context, method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := c.request(ctx, method, path, r)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("immich: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("immich: %s %s: %s %s", method, path, resp.Status, strings.TrimSpace(string(msg)))
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out); err != nil {
		return fmt.Errorf("immich: %s %s: %w", method, path, err)
	}
	return nil
}

func (c *Client) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if c.Base == "" {
		return nil, fmt.Errorf("immich: no server set")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, body)
	if err != nil {
		return nil, fmt.Errorf("immich: %w", err)
	}
	req.Header.Set("x-api-key", c.Key)
	req.Header.Set("Accept", "application/json")
	return req, nil
}
