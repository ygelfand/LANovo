package sendspin

import (
	"testing"
	"time"

	"github.com/Sendspin/sendspin-go/pkg/protocol"
)

func TestThePositionRunsFromTheServersTimestamp(t *testing.T) {
	now := int64(100_000_000)
	clock := func() int64 { return now }

	cases := []struct {
		name  string
		stamp int64
		speed int
		want  time.Duration
	}{
		{"stamped two seconds ago", now - 2_000_000, 1000, 12 * time.Second},
		{"stamped half a second ahead", now + 500_000, 1000, 9500 * time.Millisecond},
		{"paused", now - 2_000_000, 0, 10 * time.Second},
	}
	for _, c := range cases {
		p := &Player{}
		p.moved(protocol.ProgressState{TrackProgress: 10_000, TrackDuration: 200_000, PlaybackSpeed: c.speed}, c.stamp, clock)
		if got := p.reached(true); got != c.want {
			t.Errorf("%s: at %v, want %v", c.name, got, c.want)
		}
	}
}
