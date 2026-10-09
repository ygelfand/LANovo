package speaker

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

type caught struct{ got []int16 }

func (c *caught) Offer(mix []int16) { c.got = append(c.got, mix...) }

func TestTheTapHearsTheMixAtTheVolume(t *testing.T) {
	for _, tc := range []struct {
		volume float32
		want   int16
	}{{1, 1000}, {0.5, 500}, {0, 0}} {
		s := &Speaker{}
		s.SetVolume(tc.volume)
		queued := make([]int16, period*Channels)
		for i := range queued {
			queued[i] = 1000
		}
		s.bus.Push(schema.StreamMedia, queued)

		c := &caught{}
		s.SetTap(c)
		s.fill(make([]byte, period*Channels*Bits/8))

		if len(c.got) != period*Channels {
			t.Fatalf("the tap got %d samples, want a period's %d", len(c.got), period*Channels)
		}
		if c.got[0] != tc.want {
			t.Errorf("the tap heard %d at volume %v, want %d", c.got[0], tc.volume, tc.want)
		}
	}
}

func TestNoTapNoOffer(t *testing.T) {
	s := &Speaker{}
	c := &caught{}
	s.SetTap(c)
	s.SetTap(nil)
	s.fill(make([]byte, period*Channels*Bits/8))
	if len(c.got) != 0 {
		t.Errorf("a cleared tap still got %d samples", len(c.got))
	}
}
