package visuals

import "math"

type loudest struct {
	most float64
}

func (l *loudest) hear(f []int16) {
	for _, s := range f {
		l.most = math.Max(l.most, math.Abs(float64(s))/32768)
	}
}

func (l *loudest) reset() { l.most = 0 }
