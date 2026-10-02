package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// What a TV screen calls itself asking for the queue around a video, from yt-cast-receiver.
const (
	tvClient    = "TVHTML5"
	tvVersion   = "7.20230405.08.01"
	tvUserAgent = "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3770.142 Safari/537.36; SMART-TV; Tizen 4.0,gzip(gfe)"
)

// nextURL is YouTube's watch-next endpoint.
var nextURL = "https://www.youtube.com/youtubei/v1/next"

// Entry is a video in the phone's queue.
type Entry struct {
	ID     string
	Title  string
	Artist string
	Index  int
	List   string
	Params string
}

// Position is where in the phone's queue a video is: the list, its index there, and its params.
type Position struct {
	Video  string
	List   string
	Index  int
	Params string
}

// neighbours asks YouTube what comes either side of a video in the phone's queue. The phone's device
// id is what lets it see a remote queue at all.
func neighbours(ctx context.Context, c *http.Client, at Position, phones []string) (prev, next *Entry, err error) {
	devices := make([]map[string]string, 0, len(phones))
	for _, p := range phones {
		devices = append(devices, map[string]string{"deviceId": p})
	}
	body := map[string]any{
		"context": map[string]any{
			"client": map[string]string{"clientName": tvClient, "clientVersion": tvVersion, "hl": "en", "gl": "US"},
		},
		"videoId":           at.Video,
		"enableMdxAutoplay": true,
		"isMdxPlayback":     true,
		"mdxContext": map[string]any{
			"mdxReceiverContext": map[string]any{"mdxConnectedDevices": devices},
		},
	}
	if at.List != "" {
		body["playlistId"] = at.List
		body["playlistIndex"] = at.Index
	}
	if at.Params != "" {
		body["params"] = at.Params
	}
	raw, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, nextURL, bytes.NewReader(raw))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", tvUserAgent)

	resp, err := c.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("youtube: the queue around %s: %s", at.Video, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, nil, err
	}
	prev, next = parseNeighbours(data)
	return prev, next, nil
}

type endpoint struct {
	WatchEndpoint struct {
		VideoID    string `json:"videoId"`
		PlaylistID string `json:"playlistId"`
		Index      int    `json:"index"`
		Params     string `json:"params"`
	} `json:"watchEndpoint"`
}

type videoRenderer struct {
	AutoplayEndpointRenderer struct {
		Endpoint endpoint `json:"endpoint"`
	} `json:"autoplayEndpointRenderer"`
	AutoplayVideoWrapperRenderer struct {
		PrimaryEndpointRenderer struct {
			AutoplayEndpointRenderer struct {
				Endpoint endpoint `json:"endpoint"`
			} `json:"autoplayEndpointRenderer"`
		} `json:"primaryEndpointRenderer"`
	} `json:"autoplayVideoWrapperRenderer"`
}

type nextResponse struct {
	Contents struct {
		SingleColumnWatchNextResults struct {
			Autoplay struct {
				Autoplay struct {
					Sets []struct {
						PreviousVideoRenderer videoRenderer `json:"previousVideoRenderer"`
						NextVideoRenderer     videoRenderer `json:"nextVideoRenderer"`
					} `json:"sets"`
				} `json:"autoplay"`
			} `json:"autoplay"`
		} `json:"singleColumnWatchNextResults"`
	} `json:"contents"`
}

// parseNeighbours takes the previous and next videos out of a watch-next answer.
func parseNeighbours(data []byte) (prev, next *Entry) {
	var r nextResponse
	if json.Unmarshal(data, &r) != nil {
		return nil, nil
	}
	sets := r.Contents.SingleColumnWatchNextResults.Autoplay.Autoplay.Sets
	if len(sets) == 0 {
		return nil, nil
	}
	return entry(sets[0].PreviousVideoRenderer), entry(sets[0].NextVideoRenderer)
}

func entry(v videoRenderer) *Entry {
	e := v.AutoplayEndpointRenderer.Endpoint.WatchEndpoint
	if e.VideoID == "" {
		e = v.AutoplayVideoWrapperRenderer.PrimaryEndpointRenderer.AutoplayEndpointRenderer.Endpoint.WatchEndpoint
	}
	if e.VideoID == "" {
		return nil
	}
	return &Entry{ID: e.VideoID, List: e.PlaylistID, Index: e.Index, Params: e.Params}
}
