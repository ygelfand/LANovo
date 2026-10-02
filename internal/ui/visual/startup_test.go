package visual

import (
	"fmt"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/ui"
)

func BenchmarkStartup(b *testing.B) {
	sizes := []ui.Rect{{W: 1200, H: 1920}, {W: 1920, H: 1200}}
	x := Input{Now: time.Second, Dt: time.Second / 60}
	for _, k := range Built() {
		for _, in := range sizes {
			b.Run(fmt.Sprintf("%s/%dx%d", k, in.W, in.H), func(b *testing.B) {
				for range b.N {
					v := New(k)
					if err := v.Shade(&fakeGL{}, true, in, x); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
