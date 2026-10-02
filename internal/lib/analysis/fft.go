package analysis

import "math"

func Transform(re, im []float64) {
	n := len(re)
	if n < 2 {
		return
	}

	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j |= bit

		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}

	for length := 2; length <= n; length <<= 1 {
		angle := -2 * math.Pi / float64(length)
		wr, wi := math.Cos(angle), math.Sin(angle)

		for i := 0; i < n; i += length {
			cr, ci := 1.0, 0.0

			for j := range length / 2 {
				ar, ai := re[i+j], im[i+j]
				br, bi := re[i+j+length/2], im[i+j+length/2]

				tr := br*cr - bi*ci
				ti := br*ci + bi*cr

				re[i+j], im[i+j] = ar+tr, ai+ti
				re[i+j+length/2], im[i+j+length/2] = ar-tr, ai-ti

				cr, ci = cr*wr-ci*wi, cr*wi+ci*wr
			}
		}
	}
}
