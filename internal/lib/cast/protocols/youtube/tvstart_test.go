package youtube

import (
	"testing"
	"time"
)

func TestYouTubeTVStartsWhereThePhoneAsked(t *testing.T) {
	now := time.Unix(1_790_000_000, 0)
	wall := func(ago time.Duration) time.Duration { return time.Duration(now.Add(-ago).Unix()) * time.Second }
	lead := max(int64(liveLead()/tvSegment), 1)
	const head, oldest = 10_000, 9_000

	cases := []struct {
		name string
		at   time.Duration
		want int64
	}{
		{"no position", 0, head - lead},
		{"a position within the track, not a wall time", 30 * time.Second, head - lead},
		{"a minute behind live", wall(time.Minute), head - 12},
		{"closer to live than the lead", wall(time.Second), head - lead},
		{"further back than the window keeps", wall(3 * time.Hour), oldest},
	}
	for _, c := range cases {
		if got := tvStart(head, oldest, c.at, now); got != c.want {
			t.Errorf("%s: sq %d, want %d", c.name, got, c.want)
		}
	}
}
