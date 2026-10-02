package cenc

import "testing"

func FuzzSegmentsNeverPanic(f *testing.F) {
	for _, init := range [][]byte{videoInit(90000), audioInit()} {
		for _, senc := range []bool{false, true} {
			seg := segmentOf(0, twoPictures(), senc, !senc, []int{1, 2})
			f.Add(init, seg)
			f.Add(init, append(append([]byte{}, init...), seg...))
		}
	}
	f.Fuzz(func(t *testing.T, init, seg []byte) {
		track, err := ParseInit(init)
		if err != nil {
			track = Track{Scheme: SchemeCENC, IVSize: 8, LengthSize: 4}
		}
		samples, _, _ := ParseFragment(track, seg)
		for _, s := range samples {
			track.LengthSize = 4
			track.AnnexB(s)
		}
		Parse(seg)
	})
}
