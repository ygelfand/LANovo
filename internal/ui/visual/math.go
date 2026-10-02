package visual

import "math"

func smoothstep(a, b, x float64) float64 {
	t := math.Max(0, math.Min(1, (x-a)/(b-a)))
	return t * t * (3 - 2*t)
}

func wrap(x, period float64) float32 {
	x = math.Mod(x, period)
	if x < 0 {
		x += period
	}
	return float32(x)
}

func clamp01(v float64) float64 { return max(0, min(v, 1)) }

func follow(cur, target, up, down, dt float64) float64 {
	k := down
	if target > cur {
		k = up
	}
	return cur + (target-cur)*(1-pow(1-k, dt*30))
}

func cellHash(x, y int, salt uint32) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263 + salt*2246822519
	h = (h ^ h>>13) * 1274126177
	return float64(h^h>>16) / 4294967295
}
