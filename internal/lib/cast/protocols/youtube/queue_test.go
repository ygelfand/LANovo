package youtube

import (
	"net/http"
	"os"
	"testing"
	"time"
)

func TestNeighboursComeOutOfAWatchNextAnswer(t *testing.T) {
	answer := []byte(`{"contents":{"singleColumnWatchNextResults":{"autoplay":{"autoplay":{"sets":[{
		"previousVideoRenderer":{"autoplayEndpointRenderer":{"endpoint":{"watchEndpoint":{"videoId":"prev1","playlistId":"RQx","index":12}}}},
		"nextVideoRenderer":{"autoplayEndpointRenderer":{"endpoint":{"watchEndpoint":{"videoId":"next1","playlistId":"RQx","index":14,"params":"p"}}}}
	}]}}}}}`)
	prev, next := parseNeighbours(answer)
	if prev == nil || prev.ID != "prev1" || prev.Index != 12 {
		t.Errorf("previous is %+v", prev)
	}
	if next == nil || next.ID != "next1" || next.Index != 14 || next.Params != "p" || next.List != "RQx" {
		t.Errorf("next is %+v", next)
	}
	if p, n := parseNeighbours([]byte(`{}`)); p != nil || n != nil {
		t.Error("an answer with no queue yielded neighbours")
	}
}

// Against YouTube itself, so only when asked: LANOVO_LIVE=1.
func TestTheQueueAroundAMixVideo(t *testing.T) {
	if os.Getenv("LANOVO_LIVE") == "" {
		t.Skip("set LANOVO_LIVE=1 to ask YouTube")
	}
	c := &http.Client{Timeout: 30 * time.Second}
	prev, next, err := neighbours(t.Context(), c, Position{Video: "Y7nDX15KJ5M", List: "RDAMVMY7nDX15KJ5M", Index: 0}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil {
		t.Fatal("no next video in the mix")
	}
	t.Logf("previous %+v, next %+v", prev, next)
}
