package visual

import (
	_ "embed"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
)

func init() { register(LensFlares, func() Visual { return &lensFlares{tint: [3]float64{1, 1, 1}} }) }

var lensGhostBands = [6][2]int{{0, 4}, {4, 8}, {8, 12}, {12, 17}, {17, 23}, {23, 31}}

type lensFlares struct {
	st stage
	fr framer

	bright, streak, pulse, spin float64
	tint                        [3]float64
	ghost                       [6]float64
}

func (v *lensFlares) advance(f frame) {
	dt := f.dt
	talk := f.state == listening || f.state == responding
	want := [3]float64{0.95, 0.93, 0.88}
	switch f.state {
	case listening:
		want = [3]float64{0.55, 0.8, 1}
	case responding:
		want = [3]float64{1, 0.72, 0.4}
	}
	for i := range v.tint {
		v.tint[i] = follow(v.tint[i], want[i], 0.12, 0.12, dt)
	}
	target := 0.35 + 0.05*math.Sin(f.t*0.6)
	if talk {
		target = 0.45 + 0.9*f.level
	}
	v.bright = follow(v.bright, target, 0.4, 0.1, dt)
	v.streak = follow(v.streak, 0.35+1.1*f.level, 0.3, 0.08, dt)
	if f.onset && talk {
		v.pulse = 1
	}
	v.pulse = math.Max(0, v.pulse-dt*2.5)
	v.spin += dt * (0.05 + 0.35*f.level)
	for k, b := range lensGhostBands {
		s := 0.0
		for i := b[0]; i < b[1]; i++ {
			s += f.bands[i]
		}
		s /= float64(b[1] - b[0])
		if !talk {
			s = 0.2
		}
		v.ghost[k] = follow(v.ghost[k], clamp01(s), 0.35, 0.1, dt)
	}
}

//go:embed lensflares.glsl
var lensFlaresShader string

func (v *lensFlares) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(lensFlaresShader, Light); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	v.advance(f)
	W, H, t := float64(v.st.w), float64(v.st.h), f.t
	s := math.Min(W, H)
	sx := W * (0.5 + 0.34*math.Sin(t*0.071+2.2))
	sy := H * (0.3 + 0.13*math.Sin(t*0.053+1.3))
	base := [6]float64{0.02, 0.055, 0.03, 0.09, 0.04, 0.13}
	vals := make([]float32, 26)
	vals[0], vals[1], vals[2], vals[3] = float32(W), float32(s), float32(sx), float32(sy)
	vals[4], vals[5] = float32(W/2), float32(H/2)
	vals[6], vals[7], vals[8], vals[9] = float32(v.bright*(1+0.6*v.pulse)), float32(v.streak), float32(math.Cos(v.spin)), float32(math.Sin(v.spin))
	vals[21], vals[22] = float32(math.Cos(0.7*v.spin)), float32(math.Sin(0.7*v.spin))
	vals[23], vals[24] = float32((W/2-sx)/s), float32((H/2-sy)/s)
	vals[25] = float32(4 * v.st.u)
	vals[10], vals[11], vals[12] = float32(v.tint[0]), float32(v.tint[1]), float32(v.tint[2])
	vals[13] = float32(t)
	for k := range base {
		vals[14+k] = float32(base[k] * (0.7 + 0.9*v.ghost[k]))
	}
	vals[20] = float32(0.4 + 0.6*v.ghost[2])
	return g.Values(vals, float32(0.55+0.5*v.bright), 0.02, 2)
}
