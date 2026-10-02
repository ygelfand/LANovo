package visual

import (
	_ "embed"
	"math"

	"github.com/ygelfand/LANovo/internal/lib/analysis"
	"github.com/ygelfand/LANovo/internal/ui"
)

// Ported from anchorapp100/ha-visualisations src/21-digital-vu.js (MIT).

func init() { register(DigitalVU, func() Visual { return &digitalVU{} }) }

const (
	dvuN   = 32
	dvuSeg = 22
)

type digitalVU struct {
	st stage
	fr framer

	h, pk, pkT [dvuN]float64
}

func (v *digitalVU) advance(f frame) {
	t, dt := f.t, f.dt
	talk := f.state == listening || f.state == responding
	for i := range dvuN {
		target := 0.0
		if talk {
			bi := float64(i) / (dvuN - 1) * 30
			b0 := int(bi)
			bv := f.bands[b0]*(1-(bi-float64(b0))) + f.bands[min(analysis.Bands-1, b0+1)]*(bi-float64(b0))
			target = clamp01((bv - 0.28) / 0.72 * (0.9 + 0.35*float64(i)/(dvuN-1)) * (0.55 + 0.55*f.level))
		}
		if target > v.h[i] {
			v.h[i] += (target - v.h[i]) * (1 - math.Pow(0.2, dt*30))
		} else {
			v.h[i] = math.Max(target, v.h[i]-dt*1.3)
		}
		if v.h[i] >= v.pk[i] {
			v.pk[i], v.pkT[i] = v.h[i], t
		} else if t-v.pkT[i] > 0.6 {
			v.pk[i] = math.Max(v.h[i], v.pk[i]-dt*0.5)
		}
	}

}

//go:embed digitalvu.glsl
var digitalVUShader string

func (v *digitalVU) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(digitalVUShader, Light); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	v.advance(f)
	areaW := math.Min(W*0.86, H*1.95)
	top, base := H*0.1, H*0.58
	segH := (base - top) / dvuSeg
	vals := make([]float32, 73)
	for i := range dvuN {
		vals[i], vals[32+i] = float32(v.h[i]), float32(v.pk[i])
	}
	vals[64], vals[65], vals[66], vals[67] = float32((W-areaW)/2), float32(areaW/dvuN), float32(base), float32(segH)
	vals[68], vals[69], vals[70], vals[71], vals[72] = float32(top), float32(math.Max(4*u, segH*0.9)), float32(W), float32(H), float32(u)
	return g.Values(vals, float32(0.32+0.3*f.level), 0.012, 2)
}
